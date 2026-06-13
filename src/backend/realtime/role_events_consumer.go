package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"

	"voice/backend/pkg/natslog"
)

const jsStreamRoleEvents = "role_events"

func roleConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_role"
}

type roleEventJSON struct {
	SpaceID     string `json:"space_id,omitempty"`
	RoleID      string `json:"role_id,omitempty"`
	ProfileID   string `json:"profile_id,omitempty"`
	Name        string `json:"name,omitempty"`
	ChatID      string `json:"chat_id,omitempty"`
	VoiceRoomID string `json:"voice_room_id,omitempty"`
}

func roleEventToFanout(subject string, data []byte) (profileID, chatID string, env fanoutEnvelope, ok bool) {
	var payload roleEventJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", "", fanoutEnvelope{}, false
	}
	d, err := json.Marshal(map[string]string{
		"space_id":      payload.SpaceID,
		"role_id":       payload.RoleID,
		"profile_id":    payload.ProfileID,
		"name":          payload.Name,
		"chat_id":       payload.ChatID,
		"voice_room_id": payload.VoiceRoomID,
		"subject":       subject,
	})
	if err != nil {
		return "", "", fanoutEnvelope{}, false
	}
	env = fanoutEnvelope{Op: "role_update", D: d}
	switch {
	case strings.HasSuffix(subject, "role.assigned"),
		strings.HasSuffix(subject, "role.revoked"):
		return payload.ProfileID, "", env, payload.ProfileID != ""
	case strings.HasSuffix(subject, "role.chat_override_set"):
		return "", payload.ChatID, env, payload.ChatID != ""
	case strings.HasSuffix(subject, "role.created"),
		strings.HasSuffix(subject, "role.updated"),
		strings.HasSuffix(subject, "role.deleted"),
		strings.HasSuffix(subject, "role.voice_override_set"):
		return "", "", env, payload.SpaceID != "" || payload.ChatID != ""
	default:
		return "", "", fanoutEnvelope{}, false
	}
}

func subscribeRoleEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	durable := roleConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		profileID, chatID, fe, ok := roleEventToFanout(msg.Subject, msg.Data)
		if !ok {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown role event payload")
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "role event consumed")
		reqID := natslog.RequestIDFromMsg(msg)
		switch {
		case profileID != "":
			hub.broadcastToProfile(profileID, fe, logger, reqID)
		case chatID != "":
			hub.broadcastToChat(chatID, fe, logger, reqID)
		default:
			// Space-wide role metadata: fan-out to subscribers of any chat in payload.
			if chatID := strings.TrimSpace(extractRoleChatID(msg.Data)); chatID != "" {
				hub.broadcastToChat(chatID, fe, logger, reqID)
			}
		}
	}
	sub, err := js.Subscribe("role.>", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamRoleEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamRoleEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe role.events: %w", err)
		}
	}
	return sub, nil
}

func extractRoleChatID(data []byte) string {
	var payload roleEventJSON
	if err := json.Unmarshal(data, &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.ChatID)
}

func runRoleEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("role events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-realtime-role"),
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
	sub, err := subscribeRoleEvents(js, hub, instanceID, logger)
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	<-ctx.Done()
	return ctx.Err()
}
