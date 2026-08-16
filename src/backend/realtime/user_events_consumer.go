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

const jsStreamUserEvents = "user_events"

func userConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_user"
}

// presenceWireForObservers maps stored presence to what peers should see (presence.md invisible).
func presenceWireForObservers(status, customStatus string) (wireStatus, wireCustom string) {
	st := strings.TrimSpace(strings.ToLower(status))
	if st == "invisible" {
		return "", ""
	}
	return status, customStatus
}

func presenceChangeFanoutPayload(profileID, status, customStatus string) (json.RawMessage, error) {
	wireStatus, wireCustom := presenceWireForObservers(status, customStatus)
	return json.Marshal(map[string]any{
		"profile_id":    profileID,
		"status":        wireStatus,
		"custom_status": wireCustom,
	})
}

func dispatchPresenceChangeToFriends(hub *wsHub, friends friendLister, profileID, status string, logger *slog.Logger, requestID string) {
	if hub == nil || strings.TrimSpace(profileID) == "" {
		return
	}
	d, err := presenceChangeFanoutPayload(profileID, status, "")
	if err != nil {
		return
	}
	env := fanoutEnvelope{Op: "presence_update", D: d}
	if friends == nil {
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
	for _, friendID := range ids {
		if friendID == "" || friendID == profileID {
			continue
		}
		hub.broadcastToProfile(friendID, env, logger, requestID)
	}
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

func subscribeUserEvents(js nats.JetStreamContext, hub *wsHub, friends friendLister, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
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
		dispatchPresenceChangeToFriends(hub, friends, profileID, status, logger, natslog.RequestIDFromMsg(msg))
	}
	sub, err := js.Subscribe("user.presence_changed", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamUserEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamUserEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe user.events: %w", err)
		}
	}
	return sub, nil
}

func runUserEventsConsumer(ctx context.Context, hub *wsHub, friends friendLister, natsURL, instanceID string, logger *slog.Logger) error {
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
		return subscribeUserEvents(js, hub, friends, instanceID, logger)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("user.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
