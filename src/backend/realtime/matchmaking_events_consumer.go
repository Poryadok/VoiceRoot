package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"voice/backend/pkg/natslog"
)

const jsStreamMatchmakingEvents = "matchmaking_events"

func matchmakingConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_mm"
}

func subscribeMatchmakingEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	durable := matchmakingConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		switch {
		case strings.HasSuffix(msg.Subject, "mm.match_found"),
			strings.HasSuffix(msg.Subject, "mm.match_completed"),
			strings.HasSuffix(msg.Subject, "mm.search_nudge"),
			strings.HasSuffix(msg.Subject, "mm.search_timeout"):
			natslog.LogConsume(logger, msg, slog.LevelInfo, "matchmaking event consumed")
			dispatchMatchmakingStreamEvent(hub, msg.Data)
		default:
			natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown matchmaking event payload")
		}
	}
	sub, err := js.Subscribe("mm.>", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamMatchmakingEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamMatchmakingEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe matchmaking.events: %w", err)
		}
	}
	return sub, nil
}

func runMatchmakingEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("matchmaking events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-realtime-matchmaking"),
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

	sub, err := subscribeJetStreamWithRetry(ctx, "realtime matchmaking.events", func() (*nats.Subscription, error) {
		return subscribeMatchmakingEvents(js, hub, instanceID, logger)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("matchmaking.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
