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

const jsStreamSubscriptionEvents = "subscription_events"

func runSubscriptionEventsConsumer(
	ctx context.Context,
	natsURL string,
	logger *slog.Logger,
) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("subscription notification consumer: missing NATS_URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-subscription"),
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

	handler := &consumer.SubscriptionEventHandler{}
	durable := consumer.SharedDurable("subscription")

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.SubscriptionStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "subscription event unmarshal failed")
			_ = msg.Ack()
			return
		}
		if routeSubscriptionNotification(handler, &env) {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "subscription notification event consumed")
		}
		_ = msg.Ack()
	}

	sub, err := js.Subscribe("subscription.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamSubscriptionEvents),
		nats.DeliverNew(),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamSubscriptionEvents, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("jetstream subscribe subscription.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("subscription.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func routeSubscriptionNotification(handler *consumer.SubscriptionEventHandler, env *eventsv1.SubscriptionStreamEvent) bool {
	if handler == nil || env == nil {
		return false
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.SubscriptionStreamEvent_GraceReminder:
		res := handler.HandleGraceReminder(context.Background(), p.GraceReminder)
		return res.Handled
	default:
		return false
	}
}
