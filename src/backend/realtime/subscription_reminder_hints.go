package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"voice/backend/pkg/reminderhint"
)

func reminderHintDurable(instance string) string {
	return "rt_" + strings.ReplaceAll(strings.TrimSpace(instance), "-", "") + "_subscription_reminder"
}

// dispatchReminderHint handles only the exact trusted Notification subject.
// It returns recognition, not a durable insertion or client delivery receipt.
func dispatchReminderHint(hub *wsHub, subject string, raw []byte, now time.Time) bool {
	if hub == nil || subject != reminderhint.Subject {
		return false
	}
	hint, err := reminderhint.Decode(raw, now)
	if err != nil {
		return false
	}
	data, err := hint.ClientData()
	if err != nil {
		return false
	}
	// The ordinary hub enqueue AND actual WS write retain current session-epoch
	// guards. No chat subscription or another profile on the account is a target.
	hub.broadcastToProfile(hint.TargetProfileID, fanoutEnvelope{Op: "notification", D: data}, nil, "")
	return true
}

func subscribeReminderHints(js nats.JetStreamContext, hub *wsHub, instance string) (*nats.Subscription, error) {
	durable := reminderHintDurable(instance)
	deliver := realtimeConsumerDeliverSubject(instance, "subscription_reminder")
	if err := validateRealtimeConsumerConfig(js, reminderhint.Stream, durable, reminderhint.Subject, deliver); err != nil {
		return nil, err
	}
	return js.Subscribe(reminderhint.Subject, func(msg *nats.Msg) {
		dispatchReminderHint(hub, msg.Subject, msg.Data, time.Now())
		// Malformed/expired hints intentionally make no effect. Redelivery may
		// duplicate hints; client collapse is by the same logical NotificationID.
		_ = msg.Ack()
	}, nats.Bind(reminderhint.Stream, durable), nats.ManualAck())
}

func runReminderHints(ctx context.Context, hub *wsHub, natsURL, instance string, logger *slog.Logger, shared ...*nats.Conn) error {
	if hub == nil || strings.TrimSpace(natsURL) == "" || strings.TrimSpace(instance) == "" {
		return fmt.Errorf("reminder hint consumer missing configuration")
	}
	nc, release, err := consumerNATSConnection(natsURL, "voice-realtime-subscription-reminder", shared...)
	if err != nil {
		return err
	}
	defer release()
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	sub, err := subscribeJetStreamWithRetry(ctx, "subscription reminder hints", func() (*nats.Subscription, error) { return subscribeReminderHints(js, hub, instance) })
	if err != nil {
		return err
	}
	defer func() { _ = sub.Unsubscribe() }()
	markRealtimeConsumerBound(ctx)
	<-ctx.Done()
	return ctx.Err()
}
