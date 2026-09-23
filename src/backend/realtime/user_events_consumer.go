package main

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
)

const jsStreamUserEvents = "user_events"

func userConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_user"
}

func dispatchPresenceChangeToFriends(hub *wsHub, friends friendLister, viewer presenceViewer, profileID, status string, logger *slog.Logger, requestID string) {
	if hub == nil || strings.TrimSpace(profileID) == "" {
		return
	}
	if friends == nil || viewer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ids, err := friends.ListFriendProfileIDs(ctx, profileID)
	if err != nil {
		if logger != nil {
			logger.Warn("presence friend list failed",
				slog.String("profile_id", profileID),
				slog.String("error", err.Error()),
			)
		}
		return
	}
	friendIDs := make([]string, 0, len(ids))
	for _, friendID := range ids {
		if friendID == "" || friendID == profileID {
			continue
		}
		friendIDs = append(friendIDs, friendID)
	}
	hub.broadcastPresenceToProfiles(friendIDs, viewer, profileID, status, "", logger, requestID)
}

func userEventBytesToPresence(data []byte) (profileID, status string, ok bool) {
	var e eventsv1.UserStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return "", "", false
	}
	pc := e.GetPresenceChange()
	if pc == nil || strings.TrimSpace(pc.GetProfileId()) == "" {
		return "", "", false
	}
	return pc.GetProfileId(), pc.GetStatus(), true
}

func userEventLogAttrs(data []byte) []slog.Attr {
	var env eventsv1.UserStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return nil
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if pc := env.GetPresenceChange(); pc != nil {
		attrs = append(attrs,
			slog.String("profile_id", pc.GetProfileId()),
			slog.String("status", pc.GetStatus()),
		)
	}
	return attrs
}

func subscribeUserEvents(js nats.JetStreamContext, hub *wsHub, friends friendLister, viewer presenceViewer, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	durable := userConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		attrs := userEventLogAttrs(msg.Data)
		if !strings.HasSuffix(msg.Subject, "user.presence_changed") && msg.Subject != "user.presence_changed" {
			return
		}
		profileID, status, ok := userEventBytesToPresence(msg.Data)
		if !ok {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown user.presence_changed payload", attrs...)
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "user presence event consumed", attrs...)
		dispatchPresenceChangeToFriends(hub, friends, viewer, profileID, status, logger, natslog.RequestIDFromMsg(msg))
	}
	sub, err := js.Subscribe("user.presence_changed", handler, nats.Bind(jsStreamUserEvents, durable))
	if err != nil {
		return nil, fmt.Errorf("bind pre-provisioned user.events consumer %q: %w", durable, err)
	}
	return sub, nil
}

func runUserEventsConsumer(ctx context.Context, hub *wsHub, friends friendLister, viewer presenceViewer, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("user events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL, natsConnectOptions("voice-realtime-user-events")...)
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() { _ = nc.Drain() }()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}

	sub, err := subscribeJetStreamWithRetry(ctx, "realtime user.events", func() (*nats.Subscription, error) {
		return subscribeUserEvents(js, hub, friends, viewer, instanceID, logger)
	})
	if err != nil {
		return err
	}
	markRealtimeConsumerBound(ctx)
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("user.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
