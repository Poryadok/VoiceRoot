package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
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
	default:
		return "", fanoutEnvelope{}, false
	}
}

func subscribeMessageEvents(js nats.JetStreamContext, hub *wsHub, instanceID string) (*nats.Subscription, error) {
	durable := consumerDurableName(instanceID)
	handler := func(msg *nats.Msg) {
		if chatID, fe, ok := messageEventBytesToFanout(msg.Data); ok {
			hub.broadcastToChat(chatID, fe)
		}
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
func runMessageEventsConsumer(ctx context.Context, hub *wsHub, natsURL, instanceID string) error {
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

	sub, err := subscribeMessageEvents(js, hub, instanceID)
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			log.Printf("realtime message.events unsubscribe: %v", err)
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
