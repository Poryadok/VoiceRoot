package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/natslog"
)

const messageEventsStreamName = "message_events"
const messageActivityDurable = "chat_message_activity"
const messageActivitySubject = "message.sent"

type messageActivityStore interface {
	TouchLastMessageAt(ctx context.Context, chatID uuid.UUID, at time.Time) error
	PromoteDeclinedDMRecipients(ctx context.Context, chatID, senderProfileID uuid.UUID) error
}

func chatActivityDurableName(_ string) string { return messageActivityDurable }

func messageActivityConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		Durable:        messageActivityDurable,
		DeliverSubject: "_INBOX.voice.chat." + messageActivityDurable,
		DeliverGroup:   messageActivityDurable,
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
		FilterSubject:  messageActivitySubject,
	}
}

func validateMessageActivityDurable(js nats.JetStreamContext) error {
	want := messageActivityConsumerConfig()
	info, err := js.ConsumerInfo(messageEventsStreamName, want.Durable)
	if err != nil {
		return fmt.Errorf("inspect message activity durable: %w", err)
	}
	if info == nil || info.Stream != messageEventsStreamName || info.Name != want.Durable ||
		info.Config.Durable != want.Durable || info.Config.DeliverSubject != want.DeliverSubject ||
		info.Config.DeliverGroup != want.DeliverGroup || info.Config.DeliverPolicy != want.DeliverPolicy ||
		info.Config.AckPolicy != want.AckPolicy || info.Config.FilterSubject != want.FilterSubject ||
		len(info.Config.FilterSubjects) != 0 {
		return fmt.Errorf("message activity durable %q has incompatible configuration", want.Durable)
	}
	return nil
}

func messageActivityFromEvent(data []byte, now func() time.Time) (uuid.UUID, uuid.UUID, time.Time, bool) {
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	sent := env.GetMessageSent()
	if sent == nil || sent.GetChatId() == "" || sent.GetSenderProfileId() == "" {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	chatID, err := uuid.Parse(sent.GetChatId())
	if err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	senderID, err := uuid.Parse(sent.GetSenderProfileId())
	if err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	at := now().UTC()
	if ts := env.GetOccurredAt(); ts != nil && ts.IsValid() {
		at = ts.AsTime().UTC()
	}
	return chatID, senderID, at, true
}

func handleMessageActivity(ctx context.Context, store messageActivityStore, msg *nats.Msg, logger *slog.Logger) error {
	if msg == nil {
		natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown message activity payload")
		return nil
	}
	chatID, senderID, at, ok := messageActivityFromEvent(msg.Data, time.Now)
	if !ok {
		natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown message activity payload")
		return nil
	}
	attrs := []slog.Attr{slog.String("chat_id", chatID.String())}
	if err := store.TouchLastMessageAt(ctx, chatID, at); err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		natslog.LogConsume(logger, msg, slog.LevelWarn, "message activity touch failed", attrs...)
		return err
	}
	if err := store.PromoteDeclinedDMRecipients(ctx, chatID, senderID); err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
		natslog.LogConsume(logger, msg, slog.LevelWarn, "message activity recontact failed", attrs...)
		return err
	}
	natslog.LogConsume(logger, msg, slog.LevelInfo, "message activity touched", attrs...)
	return nil
}

func subscribeMessageActivity(ctx context.Context, js nats.JetStreamContext, store messageActivityStore, logger *slog.Logger) (*nats.Subscription, error) {
	if store == nil {
		return nil, fmt.Errorf("message activity store not configured")
	}
	if err := validateMessageActivityDurable(js); err != nil {
		return nil, err
	}
	handler := func(msg *nats.Msg) {
		if err := handleMessageActivity(ctx, store, msg, logger); err != nil {
			_ = msg.Nak()
			return
		}
		_ = msg.Ack()
	}
	sub, err := js.QueueSubscribe(messageActivitySubject, messageActivityDurable, handler,
		nats.Bind(messageEventsStreamName, messageActivityDurable), nats.ManualAck())
	if err != nil {
		return nil, fmt.Errorf("bind message activity durable: %w", err)
	}
	return sub, nil
}

func runMessageActivityConsumer(ctx context.Context, natsURL string, store messageActivityStore, logger *slog.Logger) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("message activity consumer: missing NATS URL")
	}
	for {
		err := runMessageActivityConsumerOnce(ctx, natsURL, store, logger)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if logger != nil {
			logger.Warn("message activity consumer retrying", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runMessageActivityConsumerOnce(ctx context.Context, natsURL string, store messageActivityStore, logger *slog.Logger) error {
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-chat-message-activity"),
		nats.CustomInboxPrefix("_INBOX.voice.chat"),
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
	sub, err := subscribeMessageActivity(ctx, js, store, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("message activity unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}
