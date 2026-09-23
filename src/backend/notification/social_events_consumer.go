package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/presence"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/natslog"
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
	nc, lost, err := connectNotificationConsumer(natsURL, "social")
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

	sub, err := bindPreprovisionedConsumer(js, jsStreamSocialEvents, durable, "social.>", "_INBOX.voice.notification.social", msgHandler, nats.ManualAck())
	if err != nil {
		return fmt.Errorf("bind pre-provisioned social.events consumer %q: %w", durable, err)
	}
	markNotificationConsumerBound(ctx)
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("social.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	return waitForNotificationConsumer(ctx, lost, js, jsStreamSocialEvents, durable, "social.>", "_INBOX.voice.notification.social")
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
