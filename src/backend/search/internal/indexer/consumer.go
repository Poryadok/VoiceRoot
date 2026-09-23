package indexer

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
	"voice/backend/search/internal/jetstreambind"
)

const (
	jsStreamMessageEvents = "message_events"
	// These consumer identities and filters are provisioned before Search starts.
	// Runtime only binds: it never receives JetStream management authority.
	jsSubjectMessageEvents = "message.>"
	jsDurableMessageEvents = "search-indexer-message-v1"
	jsDeliverMessageEvents = "_INBOX.voice.search.indexer.message"
	jsStreamUserEvents     = "user_events"
	jsSubjectUserEvents    = "user.>"
	jsDurableUserEvents    = "search-indexer-user-v1"
	jsDeliverUserEvents    = "_INBOX.voice.search.indexer.user"
	jsStreamChatEvents     = "chat_events"
	jsSubjectChatEvents    = ">"
	jsDurableChatEvents    = "search-indexer-chat-v1"
	jsDeliverChatEvents    = "_INBOX.voice.search.indexer.chat"
)

var searchConsumerRetryBackoff = []time.Duration{time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute}

// RunMessageEventsConsumer subscribes to message.events and updates the search index.
// It sends its setup result to ready once the push subscription is bound.
func RunMessageEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *MessageIndexer, logger *slog.Logger, metrics *ConsumerMetrics, ready chan<- error) (runErr error) {
	defer func() {
		if ready != nil {
			ready <- runErr
			close(ready)
		}
	}()
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

	handler := func(msg *nats.Msg) {
		var env eventsv1.MessageStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "message event unmarshal failed")
			jetStreamTermAck(msg, jsStreamMessageEvents, metrics, logger)
			return
		}
		err := idx.Handle(ctx, &env)
		if err != nil && logger != nil {
			logger.Warn("search index update failed", slog.Any("error", err))
		} else if err == nil {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "search message event consumed")
		}
		jetStreamConsumeAck(msg, err, jsStreamMessageEvents, metrics, logger)
	}

	sub, err := subscribeSearchConsumer(js, jsStreamMessageEvents, jsDurableMessageEvents, jsSubjectMessageEvents, jsDeliverMessageEvents, handler)
	if err != nil {
		return fmt.Errorf("jetstream subscribe message.events: %w", err)
	}
	if ready != nil {
		ready <- nil
		ready = nil
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
func RunUserEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *ProfileIndexer, logger *slog.Logger, metrics *ConsumerMetrics, ready chan<- error) error {
	return runJetStreamConsumer(ctx, natsURL, "search-user", jsStreamUserEvents, jsDurableUserEvents, jsSubjectUserEvents, jsDeliverUserEvents, func(msg *nats.Msg) {
		var env eventsv1.UserStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "user event unmarshal failed")
			jetStreamTermAck(msg, jsStreamUserEvents, metrics, logger)
			return
		}
		err := idx.Handle(ctx, &env)
		if err != nil && logger != nil {
			logger.Warn("search profile index update failed", slog.Any("error", err))
		} else if err == nil {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "search user event consumed")
		}
		jetStreamConsumeAck(msg, err, jsStreamUserEvents, metrics, logger)
	}, logger, metrics, ready)
}

// RunChatEventsConsumer subscribes to chat.events and updates chat/space projections.
func RunChatEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *ChatSpaceIndexer, logger *slog.Logger, metrics *ConsumerMetrics, ready chan<- error) error {
	return runJetStreamConsumer(ctx, natsURL, "search-chat", jsStreamChatEvents, jsDurableChatEvents, jsSubjectChatEvents, jsDeliverChatEvents, func(msg *nats.Msg) {
		var env eventsv1.ChatStreamEvent
		if err := proto.Unmarshal(msg.Data, &env); err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "chat event unmarshal failed")
			jetStreamTermAck(msg, jsStreamChatEvents, metrics, logger)
			return
		}
		err := idx.Handle(ctx, &env)
		if err != nil && logger != nil {
			logger.Warn("search chat/space index update failed", slog.Any("error", err))
		} else if err == nil {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "search chat event consumed")
		}
		jetStreamConsumeAck(msg, err, jsStreamChatEvents, metrics, logger)
	}, logger, metrics, ready)
}

func runJetStreamConsumer(ctx context.Context, natsURL, namePrefix, stream, durable, subject, deliverSubject string, handler func(*nats.Msg), logger *slog.Logger, metrics *ConsumerMetrics, ready chan<- error) (runErr error) {
	defer func() {
		if ready != nil {
			ready <- runErr
			close(ready)
		}
	}()
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

	sub, err := subscribeSearchConsumer(js, stream, durable, subject, deliverSubject, handler)
	if err != nil {
		return fmt.Errorf("jetstream subscribe %s: %w", stream, err)
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("jetstream unsubscribe failed", slog.String("stream", stream), slog.String("error", err.Error()))
		}
	}()
	if ready != nil {
		ready <- nil
		ready = nil
	}

	<-ctx.Done()
	return ctx.Err()
}

// subscribeSearchConsumer binds a pre-provisioned durable. Missing or drifted
// consumers fail the service's startup path; Search never creates or updates
// JetStream consumers at runtime.
func subscribeSearchConsumer(js nats.JetStreamContext, stream, durable, subject, deliverSubject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return jetstreambind.Bind(js, stream, durable, subject, deliverSubject, handler)
}
