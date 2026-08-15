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
	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const jsStreamModerationEvents = "moderation_events"

func notificationModerationDurable(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "notif_" + strings.ReplaceAll(id, "-", "") + "_mod"
}

func runModerationEventsConsumer(
	ctx context.Context,
	natsURL, instanceID string,
	logger *slog.Logger,
) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("moderation notification consumer: missing NATS_URL")
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

	handler := &consumer.ModerationEventHandler{}
	durable := notificationModerationDurable(instanceID)

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.ModerationStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "moderation event unmarshal failed")
			return
		}
		if routeModerationNotification(handler, &env) {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "moderation notification event consumed")
		}
	}

	sub, err := js.Subscribe("moderation.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamModerationEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamModerationEvents, durable))
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

// routeModerationNotification acknowledges handled events. Push delivery deferred until account→profile resolution exists.
func routeModerationNotification(handler *consumer.ModerationEventHandler, env *eventsv1.ModerationStreamEvent) bool {
	if handler == nil || env == nil {
		return false
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.ModerationStreamEvent_SanctionApplied:
		if p.SanctionApplied == nil {
			return false
		}
		_ = handler.HandleSanctionApplied(context.Background(), p.SanctionApplied, "")
		return true
	default:
		return false
	}
}
