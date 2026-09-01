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
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/natslog"
)

const deliveryAckStreamName = "message_events"

type deliveryCursorStore interface {
	UpsertDeliveredCursor(ctx context.Context, chatID, profileID, messageID uuid.UUID) error
}

func deliveryAckDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "msg_" + strings.ReplaceAll(id, "-", "") + "_delivery_ack"
}

func deliveryAckFromEvent(data []byte) (chatID, profileID, messageID uuid.UUID, ok bool) {
	var env eventsv1.MessageStreamEvent
	if err := proto.Unmarshal(data, &env); err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	ack := env.GetDeliveryAck()
	if ack == nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	chatID, err := uuid.Parse(strings.TrimSpace(ack.GetChatId()))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	profileID, err = uuid.Parse(strings.TrimSpace(ack.GetProfileId()))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	messageID, err = uuid.Parse(strings.TrimSpace(ack.GetMessageId()))
	if err != nil {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return chatID, profileID, messageID, true
}

func subscribeDeliveryAck(ctx context.Context, js nats.JetStreamContext, store deliveryCursorStore, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	if store == nil {
		return nil, fmt.Errorf("delivery ack store not configured")
	}
	handler := func(msg *nats.Msg) {
		chatID, profileID, messageID, ok := deliveryAckFromEvent(msg.Data)
		if !ok {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "unknown delivery_ack payload")
			_ = msg.Ack()
			return
		}
		attrs := []slog.Attr{
			slog.String("chat_id", chatID.String()),
			slog.String("profile_id", profileID.String()),
			slog.String("message_id", messageID.String()),
		}
		if err := store.UpsertDeliveredCursor(ctx, chatID, profileID, messageID); err != nil {
			attrs = append(attrs, slog.String("error", err.Error()))
			natslog.LogConsume(logger, msg, slog.LevelWarn, "delivery_ack cursor update failed", attrs...)
			_ = msg.Nak()
			return
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "delivery_ack cursor updated", attrs...)
		_ = msg.Ack()
	}
	sub, err := js.Subscribe("message.delivery_ack", handler,
		nats.Durable(deliveryAckDurableName(instanceID)),
		nats.BindStream(deliveryAckStreamName),
		nats.DeliverAll(),
		nats.ManualAck(),
	)
	if err != nil {
		sub, err = js.Subscribe("", handler, nats.Bind(deliveryAckStreamName, deliveryAckDurableName(instanceID)), nats.ManualAck())
		if err != nil {
			return nil, fmt.Errorf("jetstream subscribe message.delivery_ack: %w", err)
		}
	}
	return sub, nil
}

func runDeliveryAckConsumer(ctx context.Context, natsURL, instanceID string, store deliveryCursorStore, logger *slog.Logger) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("delivery ack consumer: missing NATS URL")
	}
	for {
		err := runDeliveryAckConsumerOnce(ctx, natsURL, instanceID, store, logger)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if logger != nil {
			logger.Warn("delivery ack consumer retrying", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runDeliveryAckConsumerOnce(ctx context.Context, natsURL, instanceID string, store deliveryCursorStore, logger *slog.Logger) error {
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-messaging-delivery-ack"),
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
	sub, err := subscribeDeliveryAck(ctx, js, store, instanceID, logger)
	if err != nil {
		return err
	}
	defer func() {
		if err := sub.Unsubscribe(); err != nil && logger != nil {
			logger.Warn("delivery_ack unsubscribe failed", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

var _ deliveryCursorStore = (*store.MessagesStore)(nil)
