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

const (
	jsStreamMessageEvents = "message_events"
)

func consumerDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "rt_" + strings.ReplaceAll(id, "-", "") + "_msg"
}

// messageEventBytesToFanout maps Messaging JetStream protobuf (message.events) to a WebSocket fan-out envelope.
// Returns ok=false for unknown payloads (e.g. reactions) or invalid data.
func messageEventBytesToFanout(data []byte) (chatID string, env fanoutEnvelope, ok bool) {
	var e eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &e); err != nil {
		return "", fanoutEnvelope{}, false
	}
	switch p := e.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		ms := p.MessageSent
		if ms == nil || ms.GetChatId() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":           ms.GetChatId(),
			"message_id":        ms.GetMessageId(),
			"sender_profile_id": ms.GetSenderProfileId(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return ms.GetChatId(), fanoutEnvelope{Op: "message_create", D: d}, true
	case *eventsv1.MessageStreamEvent_MessageEdited:
		me := p.MessageEdited
		if me == nil || me.GetChatId() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    me.GetChatId(),
			"message_id": me.GetMessageId(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return me.GetChatId(), fanoutEnvelope{Op: "message_update", D: d}, true
	case *eventsv1.MessageStreamEvent_MessageDeleted:
		md := p.MessageDeleted
		if md == nil || md.GetChatId() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    md.GetChatId(),
			"message_id": md.GetMessageId(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return md.GetChatId(), fanoutEnvelope{Op: "message_delete", D: d}, true
	case *eventsv1.MessageStreamEvent_MessageRead:
		mr := p.MessageRead
		if mr == nil || mr.GetChatId() == "" || mr.GetMessageId() == "" || mr.GetProfileId() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    mr.GetChatId(),
			"message_id": mr.GetMessageId(),
			"profile_id": mr.GetProfileId(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return mr.GetChatId(), fanoutEnvelope{Op: "message_read", D: d}, true
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		ra := p.ReactionAdded
		if ra == nil || ra.GetChatId() == "" || ra.GetMessageId() == "" || ra.GetProfileId() == "" || ra.GetEmoji() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    ra.GetChatId(),
			"message_id": ra.GetMessageId(),
			"profile_id": ra.GetProfileId(),
			"emoji":      ra.GetEmoji(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return ra.GetChatId(), fanoutEnvelope{Op: "reaction_add", D: d}, true
	case *eventsv1.MessageStreamEvent_ReactionRemoved:
		rr := p.ReactionRemoved
		if rr == nil || rr.GetChatId() == "" || rr.GetMessageId() == "" || rr.GetProfileId() == "" || rr.GetEmoji() == "" {
			return "", fanoutEnvelope{}, false
		}
		d, err := json.Marshal(map[string]string{
			"chat_id":    rr.GetChatId(),
			"message_id": rr.GetMessageId(),
			"profile_id": rr.GetProfileId(),
			"emoji":      rr.GetEmoji(),
		})
		if err != nil {
			return "", fanoutEnvelope{}, false
		}
		return rr.GetChatId(), fanoutEnvelope{Op: "reaction_remove", D: d}, true
	default:
		return "", fanoutEnvelope{}, false
	}
}

func messageEventLogAttrs(data []byte) []slog.Attr {
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return nil
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	switch p := env.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		if s := p.MessageSent; s != nil {
			attrs = append(attrs, slog.String("message_id", s.GetMessageId()), slog.String("chat_id", s.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageEdited:
		if e := p.MessageEdited; e != nil {
			attrs = append(attrs, slog.String("message_id", e.GetMessageId()), slog.String("chat_id", e.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageDeleted:
		if d := p.MessageDeleted; d != nil {
			attrs = append(attrs, slog.String("message_id", d.GetMessageId()), slog.String("chat_id", d.GetChatId()))
		}
	case *eventsv1.MessageStreamEvent_MessageRead:
		if r := p.MessageRead; r != nil {
			attrs = append(attrs, slog.String("message_id", r.GetMessageId()), slog.String("chat_id", r.GetChatId()), slog.String("profile_id", r.GetProfileId()))
		}
	case *eventsv1.MessageStreamEvent_ReactionAdded:
		if r := p.ReactionAdded; r != nil {
			attrs = append(attrs, slog.String("message_id", r.GetMessageId()), slog.String("chat_id", r.GetChatId()), slog.String("profile_id", r.GetProfileId()))
		}
	case *eventsv1.MessageStreamEvent_ReactionRemoved:
		if r := p.ReactionRemoved; r != nil {
			attrs = append(attrs, slog.String("message_id", r.GetMessageId()), slog.String("chat_id", r.GetChatId()), slog.String("profile_id", r.GetProfileId()))
		}
	}
	return attrs
}

func subscribeMessageEvents(js nats.JetStreamContext, hub *wsHub, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	durable := consumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		attrs := messageEventLogAttrs(msg.Data)
		if chatID, fe, ok := messageEventBytesToFanout(msg.Data); ok {
			natslog.LogConsume(logger, msg, slog.LevelInfo, "message event consumed", attrs...)
			hub.broadcastToChat(chatID, fe, logger, natslog.RequestIDFromMsg(msg))
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown message event payload", attrs...)
	}
	sub, err := js.Subscribe("message.>", handler,
		nats.Durable(durable),
		nats.BindStream(jsStreamMessageEvents),
		nats.DeliverNew(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(jsStreamMessageEvents, durable))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe message.events: %w", err)
		}
	}
	return sub, nil
}

// runMessageEventsConsumer subscribes to JetStream stream message_events and fans out to the local hub.
// Each Realtime instance must use its own durable (derived from instanceID) so every instance receives all events.
func runMessageEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string, logger *slog.Logger) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("message events consumer: missing hub or NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-realtime-message-events"),
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

	sub, err := subscribeJetStreamWithRetry(ctx, "realtime message.events", func() (*nats.Subscription, error) {
		return subscribeMessageEvents(js, hub, instanceID, logger)
	})
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("message.events unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
