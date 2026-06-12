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

const jsStreamMatchmakingEvents = "matchmaking_events"

func notificationMatchmakingDurable(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "notif_" + strings.ReplaceAll(id, "-", "") + "_mm"
}

func runMatchmakingEventsConsumer(
	ctx context.Context,
	natsURL, instanceID string,
	tokens *store.DeviceTokenStore,
	pusher *dispatch.PushDispatcher,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("matchmaking notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-matchmaking"),
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

	handler := &consumer.MatchmakingEventHandler{Router: delivery.DecideRouting}
	mmPusher := &dispatch.MatchmakingPusher{Tokens: tokens, Pusher: pusher}
	durable := notificationMatchmakingDurable(instanceID)

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.MatchmakingStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "matchmaking event unmarshal failed")
			return
		}
		decisions, payload, ok := routeMatchmakingNotification(handler, &env)
		if !ok {
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "matchmaking notification event consumed")
		if err := mmPusher.SendPush(context.Background(), decisions, payload); err != nil && logger != nil {
			logger.Warn("matchmaking push failed", slog.Any("error", err))
		}
	}

	sub, err := js.Subscribe("mm.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamMatchmakingEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamMatchmakingEvents, durable))
		if err != nil {
			return fmt.Errorf("jetstream subscribe matchmaking.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("matchmaking.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func routeMatchmakingNotification(h *consumer.MatchmakingEventHandler, env *eventsv1.MatchmakingStreamEvent) (map[string]delivery.DeliveryDecision, push.Payload, bool) {
	if h == nil || env == nil {
		return nil, push.Payload{}, false
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.MatchmakingStreamEvent_MatchFound:
		ev := p.MatchFound
		return h.HandleMatchFound(context.Background(), ev), push.Payload{
			Title: "Match found",
			Body:  "A match is ready — accept or decline",
			Data: map[string]string{
				"type":     string(delivery.TypeMatchFound),
				"match_id": ev.GetMatchId(),
				"game_id":  ev.GetGameId(),
				"mode":     ev.GetMode(),
			},
		}, true
	case *eventsv1.MatchmakingStreamEvent_SearchNudge:
		ev := p.SearchNudge
		return h.HandleSearchNudge(context.Background(), ev), push.Payload{
			Title: "Still searching",
			Body:  "Try adjusting your search parameters",
			Data: map[string]string{
				"type":       string(delivery.TypeSearchNudge),
				"session_id": ev.GetSessionId(),
				"game_id":    ev.GetGameId(),
				"mode":       ev.GetMode(),
			},
		}, true
	case *eventsv1.MatchmakingStreamEvent_MatchTimeout:
		ev := p.MatchTimeout
		return h.HandleSearchTimeout(context.Background(), ev), push.Payload{
			Title: "Search ended",
			Body:  "Could not find a match — try again",
			Data: map[string]string{
				"type":       string(delivery.TypeSearchTimeout),
				"session_id": ev.GetSessionId(),
				"game_id":    ev.GetGameId(),
				"mode":       ev.GetMode(),
			},
		}, true
	default:
		return nil, push.Payload{}, false
	}
}
