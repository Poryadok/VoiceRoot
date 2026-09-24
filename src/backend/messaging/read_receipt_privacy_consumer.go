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
const privacySettingsDurable = "messaging_receipt_privacy"
const privacySettingsSubject = "user.settings_changed"

type readReceiptRevocationPublisher interface {
	PublishReadReceiptRevoked(ctx context.Context, messageID, chatID, profileID, recipientProfileID string) error
}

type publicReceiptStore interface {
	ReadReceiptChatIDsForProfile(ctx context.Context, profileID uuid.UUID) ([]uuid.UUID, error)
	ClearPublicReadReceiptsForProfile(ctx context.Context, profileID uuid.UUID, chatIDs []uuid.UUID) ([]store.PublicReadReceipt, error)
}

type dmReceiptVisibilityResolver interface {
	DMReceiptVisibilityTargets(ctx context.Context, profileID uuid.UUID) (map[uuid.UUID]uuid.UUID, error)
}

func privacySettingsDurableName(_ string) string { return privacySettingsDurable }

func receiptPrivacyConsumerConfig() *nats.ConsumerConfig {
	return &nats.ConsumerConfig{
		Durable:        privacySettingsDurable,
		DeliverSubject: "_INBOX.voice.messaging." + privacySettingsDurable,
		DeliverGroup:   privacySettingsDurable,
		DeliverPolicy:  nats.DeliverAllPolicy,
		AckPolicy:      nats.AckExplicitPolicy,
		FilterSubject:  privacySettingsSubject,
	}
}

func validateReceiptPrivacyDurable(js nats.JetStreamContext) error {
	want := receiptPrivacyConsumerConfig()
	info, err := js.ConsumerInfo(privacySettingsStreamName, want.Durable)
	if err != nil {
		return fmt.Errorf("inspect receipt privacy durable: %w", err)
	}
	if info == nil || info.Stream != privacySettingsStreamName || info.Name != want.Durable ||
		info.Config.Durable != want.Durable || info.Config.DeliverSubject != want.DeliverSubject ||
		info.Config.DeliverGroup != want.DeliverGroup || info.Config.DeliverPolicy != want.DeliverPolicy ||
		info.Config.AckPolicy != want.AckPolicy || info.Config.FilterSubject != want.FilterSubject ||
		len(info.Config.FilterSubjects) != 0 {
		return fmt.Errorf("receipt privacy durable %q has incompatible configuration", want.Durable)
	}
	return nil
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

func subscribeReceiptPrivacy(ctx context.Context, js nats.JetStreamContext, receipts publicReceiptStore, targets dmReceiptVisibilityResolver, events readReceiptRevocationPublisher, logger *slog.Logger) (*nats.Subscription, error) {
	if receipts == nil || targets == nil || events == nil {
		return nil, fmt.Errorf("receipt privacy consumer dependencies not configured")
	}
	handler := func(msg *nats.Msg) {
		profileID, ok := receiptOptOutProfileID(msg.Data)
		if !ok {
			_ = msg.Ack()
			return
		}
		dmTargets, err := targets.DMReceiptVisibilityTargets(ctx, profileID)
		if err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "receipt privacy DM targets failed", slog.String("error", err.Error()))
			_ = msg.Nak()
			return
		}
		dmChats := make([]uuid.UUID, 0, len(dmTargets))
		for chatID := range dmTargets {
			dmChats = append(dmChats, chatID)
		}
		rows, err := receipts.ClearPublicReadReceiptsForProfile(ctx, profileID, dmChats)
		if err != nil {
			natslog.LogConsume(logger, msg, slog.LevelWarn, "receipt privacy revoke failed", slog.String("error", err.Error()))
			_ = msg.Nak()
			return
		}
		for _, row := range rows {
			peerID, ok := dmTargets[row.ChatID]
			if !ok {
				natslog.LogConsume(logger, msg, slog.LevelWarn, "receipt privacy target missing")
				_ = msg.Nak()
				return
			}
			recipientID := profileID
			if row.ProfileID == profileID {
				recipientID = peerID
			}
			if err := events.PublishReadReceiptRevoked(ctx, row.MessageID.String(), row.ChatID.String(), row.ProfileID.String(), recipientID.String()); err != nil {
				natslog.LogConsume(logger, msg, slog.LevelWarn, "receipt privacy revoke publish failed", slog.String("error", err.Error()))
				_ = msg.Nak()
				return
			}
		}
		natslog.LogConsume(logger, msg, slog.LevelInfo, "receipt privacy revoked", slog.String("profile_id", profileID.String()))
		_ = msg.Ack()
	}
	if err := validateReceiptPrivacyDurable(js); err != nil {
		return nil, err
	}
	sub, err := js.QueueSubscribe(privacySettingsSubject, privacySettingsDurable, handler,
		nats.Bind(privacySettingsStreamName, privacySettingsDurable), nats.ManualAck())
	if err != nil {
		return nil, fmt.Errorf("bind receipt privacy durable: %w", err)
	}
	return sub, nil
}

func runReceiptPrivacyConsumer(ctx context.Context, natsURL string, receipts publicReceiptStore, targets dmReceiptVisibilityResolver, events readReceiptRevocationPublisher, logger *slog.Logger) error {
	if strings.TrimSpace(natsURL) == "" {
		return fmt.Errorf("receipt privacy consumer: missing NATS URL")
	}
	for {
		err := runReceiptPrivacyConsumerOnce(ctx, natsURL, receipts, targets, events, logger)
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

func runReceiptPrivacyConsumerOnce(ctx context.Context, natsURL string, receipts publicReceiptStore, targets dmReceiptVisibilityResolver, events readReceiptRevocationPublisher, logger *slog.Logger) error {
	nc, err := nats.Connect(natsURL, nats.Name("voice-messaging-receipt-privacy"), nats.CustomInboxPrefix("_INBOX.voice.messaging"), nats.Timeout(10*time.Second), nats.RetryOnFailedConnect(true), nats.MaxReconnects(-1), nats.ReconnectWait(time.Second))
	if err != nil {
		return fmt.Errorf("nats connect: %w", err)
	}
	defer func() {
		if drainErr := nc.Drain(); drainErr != nil && logger != nil {
			logger.Warn("receipt privacy consumer drain", slog.String("error", drainErr.Error()))
		}
	}()
	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("jetstream: %w", err)
	}
	sub, err := subscribeReceiptPrivacy(ctx, js, receipts, targets, events, logger)
	if err != nil {
		return err
	}
	defer func() {
		if unsubscribeErr := sub.Unsubscribe(); unsubscribeErr != nil && logger != nil {
			logger.Warn("receipt privacy consumer unsubscribe", slog.String("error", unsubscribeErr.Error()))
		}
	}()
	<-ctx.Done()
	return ctx.Err()
}
