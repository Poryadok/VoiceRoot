package indexer

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
)

const (
	jsStreamMessageEvents = "message_events"
	// Messaging publishes message.sent / message.edited / … (not msg.*).
	// v4 durable: JetStream does not allow changing the initial delivery policy
	// on an existing consumer.
	jsSubjectMessageEvents   = "message.>"
	jsDurableMessagePrefix   = "search_msg_v4_"
	jsStreamUserEvents       = "user_events"
	jsStreamChatEvents       = "chat_events"
	searchConsumerMaxDeliver = -1
)

var searchConsumerRetryBackoff = []time.Duration{time.Second, 5 * time.Second, 30 * time.Second, 2 * time.Minute}

func searchConsumerConfig(durable, subject string) nats.ConsumerConfig {
	return nats.ConsumerConfig{
		Durable:        durable,
		DeliverSubject: "_INBOX.voice.search." + durable,
		FilterSubject:  subject,
		// The durable is created during Search startup, before Gateway accepts
		// traffic. Starting at the tail prevents a fresh instance from spending
		// its readiness window replaying unrelated retained history.
		DeliverPolicy: nats.DeliverNewPolicy,
		AckPolicy:     nats.AckExplicitPolicy,
		MaxDeliver:    searchConsumerMaxDeliver,
		BackOff:       append([]time.Duration(nil), searchConsumerRetryBackoff...),
	}
}

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

	durable := jsDurableMessagePrefix + strings.ReplaceAll(strings.TrimSpace(instanceID), "-", "")
	if durable == jsDurableMessagePrefix {
		durable = jsDurableMessagePrefix + "default"
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

	sub, err := subscribeSearchConsumer(js, jsStreamMessageEvents, durable, jsSubjectMessageEvents, handler)
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
func RunUserEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *ProfileIndexer, logger *slog.Logger, metrics *ConsumerMetrics) error {
	return runJetStreamConsumer(ctx, natsURL, instanceID, "search-user", jsStreamUserEvents, "user.>", func(msg *nats.Msg) {
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
	}, logger, metrics)
}

// RunChatEventsConsumer subscribes to chat.events and updates chat/space projections.
func RunChatEventsConsumer(ctx context.Context, natsURL, instanceID string, idx *ChatSpaceIndexer, logger *slog.Logger, metrics *ConsumerMetrics) error {
	return runJetStreamConsumer(ctx, natsURL, instanceID, "search-chat", jsStreamChatEvents, ">", func(msg *nats.Msg) {
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
	}, logger, metrics)
}

func runJetStreamConsumer(ctx context.Context, natsURL, instanceID, namePrefix, stream, subject string, handler func(*nats.Msg), logger *slog.Logger, metrics *ConsumerMetrics) error {
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

	sub, err := subscribeSearchConsumer(js, stream, durable, subject, handler)
	if err != nil {
		return fmt.Errorf("jetstream subscribe %s: %w", stream, err)
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("jetstream unsubscribe failed", slog.String("stream", stream), slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

// subscribeSearchConsumer prepares the durable before installing a callback so
// an old configuration cannot deliver an event under a stale retry policy.
func subscribeSearchConsumer(js nats.JetStreamContext, stream, durable, subject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	return prepareThenBindSearchConsumer(
		func() error { return ensureSearchConsumerConfig(js, stream, durable, subject) },
		func() (*nats.Subscription, error) {
			return js.Subscribe("", handler, nats.Bind(stream, durable), nats.ManualAck())
		},
	)
}

func prepareThenBindSearchConsumer(prepare func() error, bind func() (*nats.Subscription, error)) (*nats.Subscription, error) {
	if err := prepare(); err != nil {
		return nil, err
	}
	return bind()
}

// ensureSearchConsumerConfig creates or reconciles the durable before a
// callback binds to it. MaxDeliver=-1 retains transient failures until durable
// recovery; BackOff caps their retry cadence at the final configured delay.
func ensureSearchConsumerConfig(js nats.JetStreamContext, stream, durable, subject string) error {
	info, err := js.ConsumerInfo(stream, durable)
	if err != nil {
		if !errors.Is(err, nats.ErrConsumerNotFound) {
			return fmt.Errorf("jetstream consumer info %s/%s: %w", stream, durable, err)
		}
		config := searchConsumerConfig(durable, subject)
		if _, err := js.AddConsumer(stream, &config); err != nil {
			return fmt.Errorf("jetstream consumer create %s/%s: %w", stream, durable, err)
		}
		return nil
	}
	config, update, err := reconcileSearchConsumerConfig(info.Config, subject)
	if err != nil {
		return fmt.Errorf("jetstream consumer %s/%s has incompatible existing configuration", stream, durable)
	}
	if !update {
		return nil
	}
	if _, err := js.UpdateConsumer(stream, &config); err != nil {
		return fmt.Errorf("jetstream consumer retry config %s/%s: %w", stream, durable, err)
	}
	return nil
}

func reconcileSearchConsumerConfig(config nats.ConsumerConfig, subject string) (nats.ConsumerConfig, bool, error) {
	if config.FilterSubject != subject || config.AckPolicy != nats.AckExplicitPolicy || config.DeliverSubject == "" {
		return nats.ConsumerConfig{}, false, errors.New("incompatible route or acknowledgement configuration")
	}
	backoff := append([]time.Duration(nil), searchConsumerRetryBackoff...)
	if config.MaxDeliver == searchConsumerMaxDeliver && equalDurations(config.BackOff, backoff) {
		return config, false, nil
	}
	config.MaxDeliver = searchConsumerMaxDeliver
	config.BackOff = backoff
	return config, true, nil
}

func equalDurations(left, right []time.Duration) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
