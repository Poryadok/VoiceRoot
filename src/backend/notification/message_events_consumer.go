package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"voice/backend/notification/internal/chatmembers"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamMessageEvents = "message_events"

func notificationMessageDurable(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "notif_" + strings.ReplaceAll(id, "-", "") + "_msg"
}

func runMessageEventsConsumer(
	ctx context.Context,
	natsURL, instanceID string,
	tokens *store.DeviceTokenStore,
	members chatmembers.Lister,
	pusher *dispatch.MessagePusher,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("message notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-message"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	handler := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	durable := notificationMessageDurable(instanceID)

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.MessageStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "message event unmarshal failed")
			return
		}
		if err := routeMessageNotification(ctx, handler, members, pusher, &env); err != nil && logger != nil {
			logger.Warn("message push failed", slog.Any("error", err))
		} else {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "message notification event consumed")
		}
	}

	sub, err := js.Subscribe("msg.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamMessageEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamMessageEvents, durable))
		if err != nil {
			return fmt.Errorf("jetstream subscribe message.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("message.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func routeMessageNotification(
	ctx context.Context,
	handler *consumer.MessageEventHandler,
	members chatmembers.Lister,
	pusher *dispatch.MessagePusher,
	env *eventsv1.MessageStreamEvent,
) error {
	if handler == nil || pusher == nil || env == nil {
		return nil
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		ev := p.MessageSent
		if ev == nil {
			return nil
		}
		memberIDs, err := listChatMembers(ctx, members, ev.GetChatId())
		if err != nil {
			return err
		}
		if len(memberIDs) == 0 {
			return nil
		}
		raw := handler.HandleMessageSent(ctx, ev, memberIDs)
		senderID, _ := uuid.Parse(ev.GetSenderProfileId())
		decisions := enrichDecisions(ctx, pusher, raw, senderID, ev.GetChatId(), delivery.TypeNewMessage)
		payload := push.Payload{
			Title: "New message",
			Body:  "You have a new message",
			Data: map[string]string{
				"type":              string(delivery.TypeNewMessage),
				"chat_id":           ev.GetChatId(),
				"message_id":        ev.GetMessageId(),
				"sender_profile_id": ev.GetSenderProfileId(),
			},
		}
		return pusher.SendPush(ctx, decisions, delivery.DeliveryInput{
			SenderProfileID: senderID,
			ChatID:          ev.GetChatId(),
			Type:            delivery.TypeNewMessage,
		}, payload, payload.Body)
	case *eventsv1.MessageStreamEvent_MentionAdded:
		ev := p.MentionAdded
		if ev == nil {
			return nil
		}
		raw := handler.HandleMentionAdded(ctx, ev)
		senderID, _ := uuid.Parse(ev.GetSenderProfileId())
		decisions := enrichDecisions(ctx, pusher, raw, senderID, ev.GetChatId(), delivery.TypeMention)
		payload := push.Payload{
			Title: "Mention",
			Body:  "You were mentioned",
			Data: map[string]string{
				"type":              string(delivery.TypeMention),
				"chat_id":           ev.GetChatId(),
				"message_id":        ev.GetMessageId(),
				"sender_profile_id": ev.GetSenderProfileId(),
			},
		}
		return pusher.SendPush(ctx, decisions, delivery.DeliveryInput{
			SenderProfileID: senderID,
			ChatID:          ev.GetChatId(),
			Type:            delivery.TypeMention,
		}, payload, payload.Body)
	default:
		return nil
	}
}

func listChatMembers(ctx context.Context, members chatmembers.Lister, chatID string) ([]string, error) {
	if members == nil {
		return nil, nil
	}
	return members.ListMemberProfileIDs(ctx, chatID)
}

func enrichDecisions(
	ctx context.Context,
	pusher *dispatch.MessagePusher,
	raw map[string]delivery.DeliveryDecision,
	senderID uuid.UUID,
	chatID string,
	typ delivery.NotificationType,
) map[string]delivery.DeliveryDecision {
	out := make(map[string]delivery.DeliveryDecision, len(raw))
	for profileID := range raw {
		enriched, err := pusher.EnrichDecision(ctx, profileID, senderID, chatID, typ)
		if err != nil {
			continue
		}
		out[profileID] = enriched
	}
	return out
}
