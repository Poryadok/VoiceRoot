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

const jsStreamChatEvents = "chat_events"

func chatConsumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_chat"
}

// chatEventBytesToFanout maps Chat JetStream protobuf (chat.events) to WebSocket fan-out envelopes.
func chatEventBytesToFanout(data []byte) (profileID string, env fanoutEnvelope, ok bool) {
	var e eventsv1.ChatStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return "", fanoutEnvelope{}, false
	}
	switch p := e.GetPayload().(type) {
	case *eventsv1.ChatStreamEvent_ChatCreated:
		created := p.ChatCreated
		if created == nil || created.GetChatId() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id": created.GetChatId(),
			"type":    created.GetType(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		// No profile target on chat.created; fan-out via chat subscriptions when present.
		return "", fanoutEnvelope{Op: "chat_update", D: d}, true
	case *eventsv1.ChatStreamEvent_ChatMemberChanged:
		changed := p.ChatMemberChanged
		if changed == nil || changed.GetChatId() == "" || changed.GetProfileId() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    changed.GetChatId(),
			"profile_id": changed.GetProfileId(),
			"change":     changed.GetChange(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return changed.GetProfileId(), fanoutEnvelope{Op: "chat_update", D: d}, true
	default:
		return "", fanoutEnvelope{}, false
	}
}

func chatEventLogAttrs(data []byte) []slog.Attr {
	var env eventsv1.ChatStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return nil
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if created := env.GetChatCreated(); created != nil {
		attrs = append(attrs, slog.String("chat_id", created.GetChatId()))
	}
	if changed := env.GetChatMemberChanged(); changed != nil {
		attrs = append(attrs, slog.String("chat_id", changed.GetChatId()), slog.String("profile_id", changed.GetProfileId()))
	}
	return attrs
}

func subscribeChatEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	durable := chatConsumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		attrs := chatEventLogAttrs(msg.Data)
		profileID, fe, ok := chatEventBytesToFanout(msg.Data)
		if !ok {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown chat event payload", attrs...)
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "chat event consumed", attrs...)
		reqID := natslog.RequestIDFromMsg(msg)
		if profileID != "" {
			hub.broadcastToProfile(profileID, fe, logger, reqID)
			return
		}
		var body map[string]string
		if err := json.Unmarshal(fe.D, &body); err == nil {
			if chatID := strings.TrimSpace(body["chat_id"]); chatID != "" {
				hub.broadcastToChat(chatID, fe, logger, reqID)
			}
		}
	}
	sub, err := js.Subscribe("chat.>", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamChatEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamChatEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe chat.events: %w", err)
		}
	}
	return sub, nil
}

func runChatEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("chat events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-realtime-chat-events"),
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

	sub, err := subscribeJetStreamWithRetry(ctx, "realtime chat.events", func() (*nats.Subscription, error) {
		return subscribeChatEvents(js, hub, instanceID, logger)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("chat.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
