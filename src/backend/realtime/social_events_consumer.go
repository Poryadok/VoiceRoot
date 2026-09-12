package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
)

const jsStreamSocialEvents = "social_events"

func socialConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_social"
}

func socialBlockEventAccounts(data []byte) (string, string, bool) {
	var event eventsv1.SocialStreamEvent
	if err := proto.Unmarshal(data, &event); err != nil {
		return "", "", false
	}
	blocked := event.GetUserBlocked()
	if blocked == nil {
		return "", "", false
	}
	pair, ok := canonicalAccountPair(blocked.GetBlockerAccountId(), blocked.GetBlockedAccountId())
	if !ok {
		return "", "", false
	}
	return pair.first, pair.second, true
}

func subscribeSocialEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	if hub == nil {
		return nil, fmt.Errorf("social events subscriber requires hub")
	}
	durable := socialConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		accountA, accountB, ok := socialBlockEventAccounts(msg.Data)
		if !ok {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown social block event payload")
			return
		}
		hub.revokeAccountPairDMChats(accountA, accountB)
		natslog.LogConsume(logger, msg, slog.LevelInfo, "social block subscriptions revoked",
			slog.String("account_id_a", accountA),
			slog.String("account_id_b", accountB),
		)
	}
	sub, err := js.Subscribe("social.user_blocked", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamSocialEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamSocialEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe social.events: %w", err)
		}
	}
	return sub, nil
}

func runSocialEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("social events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL, natsConnectOptions("voice-realtime-social-events")...)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	sub, err := subscribeJetStreamWithRetry(ctx, "realtime social.events", func() (*nats.Subscription, error) {
		return subscribeSocialEvents(js, hub, instanceID, logger)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("social.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()
	<-ctx.Done()
	return ctx.Err()
}
