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

const jsStreamStoryEvents = "story_events"

func runStoryEventsConsumer(
	ctx context.Context,
	natsURL string,
	tokens *store.DeviceTokenStore,
	pusher *dispatch.StoryPusher,
	logger *slog.Logger,
) error {
	if tokens == nil || pusher == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("story notification consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-notification-story"),
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

	handler := &consumer.StoryEventHandler{Router: delivery.DecideRouting}
	durable := consumer.SharedDurable("story")

	msgHandler := func(msg *nats.Msg) {
		var env eventsv1.StoryStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "story event unmarshal failed")
			consumer.JetStreamTermAck(msg)
			return
		}
		err := routeStoryNotification(handler, pusher, &env)
		if err != nil && logger != nil {
			logger.Warn("story push failed", slog.Any("error", err))
		} else if err == nil {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "story notification event consumed")
		}
		consumer.JetStreamConsumeAck(msg, err)
	}

	sub, err := js.Subscribe("story.>", msgHandler,
		nats.Durable(durable),
		nats.BindStream(jsStreamStoryEvents),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", msgHandler, nats.Bind(jsStreamStoryEvents, durable), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("jetstream subscribe story.events: %w", err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("story.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func routeStoryNotification(
	handler *consumer.StoryEventHandler,
	pusher *dispatch.StoryPusher,
	env *eventsv1.StoryStreamEvent,
) error {
	if handler == nil || pusher == nil || env == nil {
		return nil
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.StoryStreamEvent_StoryCreated:
		ev := p.StoryCreated
		if ev == nil {
			return nil
		}
		decisions := handler.HandleStoryCreated(context.Background(), ev, nil)
		if len(decisions) == 0 {
			return nil
		}
		payload := push.Payload{
			Title: "Story mention",
			Body:  "You were mentioned in a story",
			Data: map[string]string{
				"type":              string(delivery.TypeMention),
				"story_id":          ev.GetStoryId(),
				"sender_profile_id": ev.GetAuthorProfileId(),
			},
		}
		return pusher.SendPush(context.Background(), decisions, payload)
	default:
		return nil
	}
}
