package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
)

const jsStreamSocialEvents = "social_events"

const (
	realtimeConsumerPreflightStartupWait = 20 * time.Second
	realtimeConsumerPreflightRetryDelay  = 250 * time.Millisecond
)

func socialConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_social"
}

func friendRequestConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_friend_request"
}

func friendRequestEventToFanout(data []byte) (string, fanoutEnvelope, bool) {
	var event eventsv1.SocialStreamEvent
	if err := proto.Unmarshal(data, &event); err != nil {
		return "", fanoutEnvelope{}, false
	}
	request := event.GetFriendRequest()
	if request == nil || strings.TrimSpace(request.GetRequestId()) == "" ||
		strings.TrimSpace(request.GetRequesterProfileId()) == "" ||
		strings.TrimSpace(request.GetTargetProfileId()) == "" {
		return "", fanoutEnvelope{}, false
	}
	payload, err := json.Marshal(map[string]string{
		"type":              "friend_request",
		"friend_request_id": request.GetRequestId(),
		"sender_profile_id": request.GetRequesterProfileId(),
	})
	if err != nil {
		return "", fanoutEnvelope{}, false
	}
	return request.GetTargetProfileId(), fanoutEnvelope{Op: "notification", D: payload}, true
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
	if err := validateRealtimeConsumerConfig(js, jsStreamSocialEvents, durable, "social.user_blocked", realtimeConsumerDeliverSubject(instanceID, "social")); err != nil {
		return nil, err
	}
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
	sub, err := js.Subscribe("social.user_blocked", handler, nats.Bind(jsStreamSocialEvents, durable))
	if err != nil {
		return nil, fmt.Errorf("bind pre-provisioned social.events consumer %q: %w", durable, err)
	}
	return sub, nil
}

func subscribeFriendRequestEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	if hub == nil {
		return nil, fmt.Errorf("friend request subscriber requires hub")
	}
	durable := friendRequestConsumerDurableName(instanceID)
	if err := validateRealtimeConsumerConfig(js, jsStreamSocialEvents, durable, "social.friend_request", realtimeConsumerDeliverSubject(instanceID, "friend_request")); err != nil {
		return nil, err
	}
	handler := func(msg *nats.Msg) {
		profileID, envelope, ok := friendRequestEventToFanout(msg.Data)
		if !ok {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "invalid friend request event payload")
			return
		}
		hub.broadcastToProfile(profileID, envelope, logger, "")
		natslog.LogConsume(logger, msg, slog.LevelInfo, "friend request delivered")
	}
	sub, err := js.Subscribe("social.friend_request", handler, nats.Bind(jsStreamSocialEvents, durable))
	if err != nil {
		return nil, fmt.Errorf("bind pre-provisioned social.events consumer %q: %w", durable, err)
	}
	return sub, nil
}

// preflightFriendRequestConsumer verifies the fixed durable through the deployed
// credential without binding to the live push consumer. Its subscribe grant
// remains an external ACL activation gate for the subsequent Realtime rollout.
func preflightFriendRequestConsumer(natsURL, instanceID string) error {
	return preflightFriendRequestConsumerWithWait(natsURL, instanceID, realtimeConsumerPreflightStartupWait)
}

func preflightFriendRequestConsumerWithWait(natsURL, instanceID string, startupWait time.Duration) error {
	if strings.TrimSpace(natsURL) == "" || strings.TrimSpace(instanceID) == "" {
		return fmt.Errorf("missing Realtime NATS preflight configuration")
	}
	nc, err := nats.Connect(natsURL, natsConnectOptions("voice-realtime-friend-request-preflight")...)
	if err != nil {
		return err
	}
	defer nc.Close()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	durable := friendRequestConsumerDurableName(instanceID)
	deadline := time.NewTimer(startupWait)
	defer deadline.Stop()
	ticker := time.NewTicker(realtimeConsumerPreflightRetryDelay)
	defer ticker.Stop()
	var lastErr error
	for {
		lastErr = validateRealtimeConsumerConfig(js, jsStreamSocialEvents, durable, "social.friend_request", realtimeConsumerDeliverSubject(instanceID, "friend_request"))
		if lastErr == nil {
			return nil
		}
		select {
		case <-deadline.C:
			return fmt.Errorf("realtime NATS friend request consumer did not become ready within %s: %w", startupWait, lastErr)
		case <-ticker.C:
		}
	}
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
	requestSub, err := subscribeJetStreamWithRetry(ctx, "realtime social.friend_request", func() (*nats.Subscription, error) {
		return subscribeFriendRequestEvents(js, hub, instanceID, logger)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := requestSub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("friend request events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()
	markRealtimeConsumerBound(ctx)
	<-ctx.Done()
	return ctx.Err()
}
