package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

const messageEventsStreamName = "message_events"

type messageActivityStore interface {
	TouchLastMessageAt(ctx context.Context, chatID uuid.UUID, at time.Time) error
}

func chatActivityDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "chat_" + strings.ReplaceAll(id, "-", "") + "_msg_activity"
}

func messageActivityFromEvent(data []byte, now func() time.Time) (uuid.UUID, time.Time, bool) {
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return uuid.Nil, time.Time{}, false
	}
	sent := env.GetMessageSent()
	if sent == nil || sent.GetChatId() == "" {
		return uuid.Nil, time.Time{}, false
	}
	chatID, err := uuid.Parse(sent.GetChatId())
	if err != nil {
		return uuid.Nil, time.Time{}, false
	}
	at := now().UTC()
	if ts := env.GetOccurredAt(); ts != nil && ts.IsValid() {
		at = ts.AsTime().UTC()
	}
	return chatID, at, true
}

func subscribeMessageActivity(ctx context.Context, js nats.JetStreamContext, store messageActivityStore, instanceID string) (*nats.Subscription, error) {
	if store == nil {
		return nil, fmt.Errorf("message activity store not configured")
	}
	handler := func(msg *nats.Msg) {
		chatID, at, ok := messageActivityFromEvent(msg.Data, time.Now)
		if !ok {
			return
		}
		if err := store.TouchLastMessageAt(ctx, chatID, at); err != nil {
			log.Printf("chat: touch last_message_at: %v", err)
		}
	}
	sub, err := js.Subscribe("message.sent", handler,
		nats.Durable(chatActivityDurableName(instanceID)),
		nats.BindStream(messageEventsStreamName),
		nats.DeliverAll(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(messageEventsStreamName, chatActivityDurableName(instanceID)))
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe message activity: %w", err)
		}
	}
	return sub, nil
}

func runMessageActivityConsumer(ctx context.Context, natsURL, instanceID string, store messageActivityStore) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("message activity consumer: missing NATS URL")
	}
	for {
		err := runMessageActivityConsumerOnce(ctx, natsURL, instanceID, store)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		log.Printf("chat: message activity consumer retrying after error: %v", err)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runMessageActivityConsumerOnce(ctx context.Context, natsURL, instanceID string, store messageActivityStore) error {
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-chat-message-activity"),
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
	sub, err := subscribeMessageActivity(ctx, js, store, instanceID)
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil {
			log.Printf("chat: message activity unsubscribe: %v", err)
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
