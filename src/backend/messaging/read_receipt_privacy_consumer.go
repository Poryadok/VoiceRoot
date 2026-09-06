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

const privacySettingsStreamName = "user_events"

type readReceiptRevocationPublisher interface {
	PublishReadReceiptRevoked(ctx context.Context, messageID, chatID, profileID, recipientProfileID string) error
}

type publicReceiptStore interface {
	ClearPublicReadReceiptsForProfile(ctx context.Context, profileID uuid.UUID) ([]store.PublicReadReceipt, error)
}

func privacySettingsDurableName(instanceID string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "msg_" + strings.ReplaceAll(id, "-", "") + "_receipt_privacy"
}

func receiptOptOutProfileID(data []byte) (uuid.UUID, bool) {
	var env eventsv1.UserStreamEvent
	if proto.Unmarshal(data, &env) != nil {
		return uuid.Nil, false
	}
	changed := env.GetSettingsChanged()
	if changed == nil || !strings.Contains(changed.GetChangedKeysJson(), `"show_read_receipts"`) || !strings.Contains(changed.GetChangedKeysJson(), `false`) {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(strings.TrimSpace(changed.GetProfileId()))
	return id, err == nil
}

func subscribeReceiptPrivacy(ctx context.Context, js nats.JetStreamContext, receipts publicReceiptStore, events readReceiptRevocationPublisher, instanceID string, logger *slog.Logger) (*nats.Subscription, error) {
	if receipts == nil || events == nil {
		return nil, fmt.Errorf("receipt privacy consumer dependencies not configured")
	}
	handler := func(msg *nats.Msg) {
		profileID, ok := receiptOptOutProfileID(msg.Data)
		if !ok {
			_ = msg.Ack()
			return
		}
		rows, err := receipts.ClearPublicReadReceiptsForProfile(ctx, profileID)
		if err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "receipt privacy revoke failed", slog.String("error", err.Error()))
			_ = msg.Nak()
			return
		}
		for _, row := range rows {
			if err := events.PublishReadReceiptRevoked(ctx, row.MessageID.String(), row.ChatID.String(), row.ProfileID.String(), row.RecipientProfileID.String()); err != nil {
				natslog.LogConsume(logger, msg, slog.LevelWarn, "receipt privacy revoke publish failed", slog.String("error", err.Error()))
				_ = msg.Nak()
				return
			}
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "receipt privacy revoked", slog.String("profile_id", profileID.String()))
		_ = msg.Ack()
	}
	return js.Subscribe("user.settings_changed", handler, nats.Durable(privacySettingsDurableName(instanceID)), nats.BindStream(privacySettingsStreamName), nats.DeliverAll(), nats.ManualAck())
}

func runReceiptPrivacyConsumer(ctx context.Context, natsURL, instanceID string, receipts publicReceiptStore, events readReceiptRevocationPublisher, logger *slog.Logger) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("receipt privacy consumer: missing NATS URL")
	}
	for {
		err := runReceiptPrivacyConsumerOnce(ctx, natsURL, instanceID, receipts, events, logger)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if logger != nil {
			logger.Warn("receipt privacy consumer retrying", slog.String("error", err.Error()))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func runReceiptPrivacyConsumerOnce(ctx context.Context, natsURL, instanceID string, receipts publicReceiptStore, events readReceiptRevocationPublisher, logger *slog.Logger) error {
	nc, err := nats.Connect(natsURL, nats.Name("voice-messaging-receipt-privacy"), nats.Timeout(10*time.Second), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer nc.Drain()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	sub, err := subscribeReceiptPrivacy(ctx, js, receipts, events, instanceID, logger)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-ctx.Done()
	return ctx.Err()
}
