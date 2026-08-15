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

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/presence"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamSocialEvents = "social_events"

func runSocialEventsConsumer(
	ctx context.Context,
	natsURL string,
	tokens *store.DeviceTokenStore,
	pusher *dispatch.PushDispatcher,
	presenceChecker presence.Checker,
	policy delivery.DeliveryPolicyLoader,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("social notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-social"),
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

	handler := &consumer.SocialEventHandler{Router: delivery.DecideRouting}
	socialPusher := &dispatch.MatchmakingPusher{Tokens: tokens, Pusher: pusher}
	durable := consumer.SharedDurable("social")

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.SocialStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "social event unmarshal failed")
			consumer.JetStreamTermAck(msg)
			return
		}
		decisions, payload, ok := routeSocialNotification(handler, &env)
		if !ok {
			consumer.JetStreamConsumeAck(msg, nil)
			return
		}
		senderID, _ := uuid.Parse(payload.Data["sender_profile_id"])
		enriched, err := dispatch.EnrichDecisions(ctx, presenceChecker, policy, decisions, senderID, "", delivery.TypeFriendReq)
		if err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "social notification enrich failed")
			consumer.JetStreamConsumeAck(msg, err)
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "social notification event consumed")
		err = socialPusher.SendPush(context.Background(), enriched, payload)
		consumer.JetStreamConsumeAck(msg, err)
		if err != nil && logger != nil {
			logger.Warn("social push failed", slog.Any("error", err))
		}
	}

	sub, err := js.Subscribe("social.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamSocialEvents),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamSocialEvents, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("jetstream subscribe social.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("social.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func routeSocialNotification(h *consumer.SocialEventHandler, env *eventsv1.SocialStreamEvent) (map[string]delivery.DeliveryDecision, push.Payload, bool) {
	if h == nil || env == nil {
		return nil, push.Payload{}, false
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.SocialStreamEvent_FriendRequest:
		ev := p.FriendRequest
		if ev == nil {
			return nil, push.Payload{}, false
		}
		return h.HandleFriendRequest(context.Background(), ev), push.Payload{
			Title: "Friend request",
			Body:  "You have a new friend request",
			Data: map[string]string{
				"type":              string(delivery.TypeFriendReq),
				"friend_request_id": ev.GetRequestId(),
				"sender_profile_id": ev.GetRequesterProfileId(),
			},
		}, true
	default:
		return nil, push.Payload{}, false
	}
}