package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamVoiceEvents = "voice_events"

func notificationVoiceDurable(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "notif_" + strings.ReplaceAll(id, "-", "") + "_voice"
}

func runVoiceEventsConsumer(
	ctx context.Context,
	natsURL, instanceID string,
	tokens *store.DeviceTokenStore,
	pusher *dispatch.PushDispatcher,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("voice notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-voice"),
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

	handler := &consumer.VoiceEventHandler{Router: delivery.DecideRouting}
	callPusher := &dispatch.CallPusher{Tokens: tokens, Pusher: pusher}
	durable := notificationVoiceDurable(instanceID)

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.VoiceStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "voice event unmarshal failed")
			return
		}
		decisions, payload, ok := routeVoiceNotification(handler, &env, false)
		if !ok {
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "voice notification event consumed")
		if err := callPusher.SendPush(context.Background(), decisions, payload); err != nil && logger != nil {
			logger.Warn("voice voip push failed", slog.Any("error", err))
		}
	}

	sub, err := js.Subscribe("voice.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamVoiceEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamVoiceEvents, durable))
		if err != nil {
			return fmt.Errorf("jetstream subscribe voice.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("voice.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func routeVoiceNotification(
	h *consumer.VoiceEventHandler,
	env *eventsv1.VoiceStreamEvent,
	isOnline bool,
) (map[string]delivery.DeliveryDecision, push.Payload, bool) {
	if h == nil || env == nil {
		return nil, push.Payload{}, false
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.VoiceStreamEvent_CallIncoming:
		ev := p.CallIncoming
		if ev == nil || ev.GetRoomId() == "" || ev.GetCalleeProfileId() == "" {
			return nil, push.Payload{}, false
		}
		expiresAt := ""
		if ev.GetExpiresAt() != nil {
			expiresAt = ev.GetExpiresAt().AsTime().UTC().Format(time.RFC3339)
		}
		return h.HandleCallIncoming(context.Background(), ev, isOnline), push.Payload{
			Data: map[string]string{
				"type":                 string(delivery.TypeIncomingCall),
				"room_id":              ev.GetRoomId(),
				"chat_id":              ev.GetChatId(),
				"initiator_profile_id": ev.GetInitiatorProfileId(),
				"callee_profile_id":    ev.GetCalleeProfileId(),
				"media_kind":           ev.GetMediaKind(),
				"livekit_room_name":    ev.GetLivekitRoomName(),
				"expires_at":           expiresAt,
			},
		}, true
	default:
		return nil, push.Payload{}, false
	}
}
