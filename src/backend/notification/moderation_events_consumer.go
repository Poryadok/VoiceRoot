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
	"voice/backend/notification/internal/s2s"
	"voice/backend/notification/internal/store"
	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamModerationEvents = "moderation_events"

func runModerationEventsConsumer(
	ctx context.Context,
	natsURL string,
	tokens *store.DeviceTokenStore,
	pusher *dispatch.PushDispatcher,
	_ presence.Checker, // reserved: presence-skip until Realtime system in-app exists
	policy delivery.DeliveryPolicyLoader,
	profiles s2s.AccountProfiles,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("moderation notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-moderation"),
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

	handler := &consumer.ModerationEventHandler{Router: delivery.DecideRouting}
	modPusher := &dispatch.MatchmakingPusher{Tokens: tokens, Pusher: pusher}
	durable := consumer.SharedDurable("moderation")

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.ModerationStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "moderation event unmarshal failed")
			consumer.JetStreamTermAck(msg)
			return
		}
		decisions, payload, ok, err := routeModerationNotification(ctx, handler, profiles, &env)
		if err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "moderation notification route failed")
			consumer.JetStreamConsumeAck(msg, err)
			return
		}
		if !ok {
			consumer.JetStreamConsumeAck(msg, nil)
			return
		}
		// Presence is intentionally skipped: Notification has no Realtime in-app
		// publisher for system/sanction, and online→InApp-only would drop delivery.
		// Matchmaking/voice use the same presence-skip pattern until in-app exists.
		enriched, err := enrichSanctionDecisions(ctx, policy, decisions)
		if err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "moderation notification enrich failed")
			consumer.JetStreamConsumeAck(msg, err)
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "moderation notification event consumed")
		err = modPusher.SendPush(context.Background(), enriched, payload)
		consumer.JetStreamConsumeAck(msg, err)
		if err != nil && logger != nil {
			logger.Warn("moderation push failed", slog.Any("error", err))
		}
	}

	sub, err := js.Subscribe("moderation.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamModerationEvents),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamModerationEvents, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("jetstream subscribe moderation.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("moderation.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

// enrichSanctionDecisions applies mute/quiet-hours policy without presence.
// Online recipients keep Push=true so sanction notices are not silently dropped
// while system in-app fan-out via Realtime is still missing.
func enrichSanctionDecisions(
	ctx context.Context,
	policy delivery.DeliveryPolicyLoader,
	decisions map[string]delivery.DeliveryDecision,
) (map[string]delivery.DeliveryDecision, error) {
	return dispatch.EnrichDecisions(ctx, presence.OfflineChecker{}, policy, decisions, uuid.Nil, "", delivery.TypeSystem)
}

// routeModerationNotification resolves account→profile and builds system push decisions.
func routeModerationNotification(
	ctx context.Context,
	handler *consumer.ModerationEventHandler,
	profiles s2s.AccountProfiles,
	env *eventsv1.ModerationStreamEvent,
) (map[string]delivery.DeliveryDecision, push.Payload, bool, error) {
	if handler == nil || env == nil {
		return nil, push.Payload{}, false, nil
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.ModerationStreamEvent_SanctionApplied:
		ev := p.SanctionApplied
		if ev == nil {
			return nil, push.Payload{}, false, nil
		}
		if !consumer.NotifySanctionType(ev.GetType()) {
			return nil, push.Payload{}, false, nil
		}
		accountID, err := uuid.Parse(strings.TrimSpace(ev.GetTargetAccountId()))
		if err != nil || accountID == uuid.Nil {
			return nil, push.Payload{}, false, nil
		}
		if profiles == nil {
			return nil, push.Payload{}, false, nil
		}
		ids, err := profiles.ProfileIDsForAccount(ctx, accountID)
		if err != nil {
			return nil, push.Payload{}, false, err
		}
		profileStrs := make([]string, 0, len(ids))
		for _, id := range ids {
			if id == uuid.Nil {
				continue
			}
			profileStrs = append(profileStrs, id.String())
		}
		decisions := handler.HandleSanctionApplied(ctx, ev, profileStrs)
		if len(decisions) == 0 {
			return nil, push.Payload{}, false, nil
		}
		return decisions, sanctionPushPayload(ev), true, nil
	default:
		return nil, push.Payload{}, false, nil
	}
}

func sanctionPushPayload(ev *eventsv1.SanctionApplied) push.Payload {
	if ev == nil {
		return push.Payload{}
	}
	return push.Payload{
		Title: "Moderation notice",
		Body:  sanctionPushBody(ev.GetType()),
		Data: map[string]string{
			"type":          string(delivery.TypeSystem),
			"sanction_id":   ev.GetSanctionId(),
			"sanction_type": ev.GetType(),
		},
	}
}

func sanctionPushBody(sanctionType string) string {
	switch strings.ToLower(strings.TrimSpace(sanctionType)) {
	case "warning":
		return "You received a warning from moderation"
	case "temp_ban":
		return "Your account has been temporarily suspended"
	case "perm_ban":
		return "Your account has been permanently banned"
	case "mm_ban":
		return "You have been banned from matchmaking"
	default:
		return "A moderation action was applied to your account"
	}
}
