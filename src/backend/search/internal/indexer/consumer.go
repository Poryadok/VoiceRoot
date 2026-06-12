package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"voice/backend/pkg/natslog"
	eventsv1 "voice.app/voice/events/v1"
)

const (
	jsStreamMessageEvents = "message_events"
	jsStreamUserEvents    = "user_events"
	jsStreamChatEvents    = "chat_events"
)

// RunMessageEventsConsumer subscribes to message.events and updates the search index.
func RunMessageEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *MessageIndexer, logger *slog.Logger) error {
	if idx == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("message events consumer: missing deps")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-search-message"),
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

	durable := "search_" + strings.ReplaceAll(strings.TrimSpace(instanceID), "-", "")
	if durable == "search_" {
		durable = "search_default"
	}

	handler := func(msg *nats.Msg) {
		var env eventsv1.MessageStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "message event unmarshal failed")
			return
		}
		if err := idx.Handle(ctx, &env); err != nil && logger != nil {
			logger.Warn("search index update failed", slog.Any("error", err))
		} else {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "search message event consumed")
		}
	}

	sub, err := js.Subscribe("msg.>", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamMessageEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamMessageEvents, durable))
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

// RunUserEventsConsumer subscribes to user.events and updates profile projections.
func RunUserEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *ProfileIndexer, logger *slog.Logger) error {
	return runJetStreamConsumer(ctx, natsURL, instanceID, "search-user", jsStreamUserEvents, "user.>", func(msg *nats.Msg) {
		var env eventsv1.UserStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "user event unmarshal failed")
			return
		}
		if err := idx.Handle(ctx, &env); err != nil && logger != nil {
			logger.Warn("search profile index update failed", slog.Any("error", err))
		} else {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "search user event consumed")
		}
	}, logger)
}

// RunChatEventsConsumer subscribes to chat.events and updates chat/space projections.
func RunChatEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *ChatSpaceIndexer, logger *slog.Logger) error {
	return runJetStreamConsumer(ctx, natsURL, instanceID, "search-chat", jsStreamChatEvents, ">", func(msg *nats.Msg) {
		var env eventsv1.ChatStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "chat event unmarshal failed")
			return
		}
		if err := idx.Handle(ctx, &env); err != nil && logger != nil {
			logger.Warn("search chat/space index update failed", slog.Any("error", err))
		} else {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "search chat event consumed")
		}
	}, logger)
}

func runJetStreamConsumer(ctx context.Context, natsURL, instanceID, namePrefix, stream, subject string, handler func(*nats.Msg), logger *slog.Logger) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("%s consumer: missing nats url", namePrefix)
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-search-"+namePrefix),
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

	durable := strings.ReplaceAll(strings.TrimSpace(instanceID), "-", "")
	if durable == "" {
		durable = "default"
	}
	durable = namePrefix + "_" + durable

	sub, err := js.Subscribe(subject, handler,
		nats.Durable(durable),
		nats.BindStream(stream),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(stream, durable))
		if err != nil {
			return fmt.Errorf("jetstream subscribe %s: %w", stream, err)
		}
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("jetstream unsubscribe failed", slog.String("stream", stream), slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
