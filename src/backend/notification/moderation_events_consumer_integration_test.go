package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/dispatch"
)

type retryingModerationProfiles struct {
	recipientID uuid.UUID
	attempts    chan uuid.UUID
	release     chan struct{}
	calls       atomic.Int32
	once        sync.Once
}

func (p *retryingModerationProfiles) unblock() { p.once.Do(func() { close(p.release) }) }

func (p *retryingModerationProfiles) ProfileIDsForAccount(ctx context.Context, accountID uuid.UUID) ([]uuid.UUID, error) {
	call := p.calls.Add(1)
	select {
	case p.attempts <- accountID:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if call == 1 {
		return nil, errors.New("temporary profile resolver failure")
	}
	select {
	case <-p.release:
		return []uuid.UUID{p.recipientID}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestModerationEventsJetStreamResolverFailureNaksThenRedeliversAndAcks(t *testing.T) {
	fixture := newNotificationConsumerRestartFixture(t)
	spec := moderationSanctionRestartSpec()
	accountID := uuid.MustParse(spec.event.(*eventsv1.ModerationStreamEvent).GetSanctionApplied().GetTargetAccountId())
	_, err := fixture.tokens.Register(context.Background(), spec.recipientID, "web", "be199-retry-"+uuid.NewString(), "fcm")
	require.NoError(t, err)
	_, err = fixture.js.AddStream(&nats.StreamConfig{Name: spec.stream, Subjects: []string{spec.filter}})
	require.NoError(t, err)
	durable := consumer.SharedDurable(spec.service)
	_, err = fixture.js.AddConsumer(spec.stream, &nats.ConsumerConfig{
		Durable: durable, FilterSubject: spec.filter, DeliverSubject: spec.deliverSubject,
		AckPolicy: nats.AckExplicitPolicy, DeliverPolicy: nats.DeliverNewPolicy,
		AckWait: 30 * time.Second,
	})
	require.NoError(t, err)

	profiles := &retryingModerationProfiles{
		recipientID: spec.recipientID, attempts: make(chan uuid.UUID, 4), release: make(chan struct{}),
	}
	t.Cleanup(profiles.unblock)
	deliveries := make(chan notificationRestartPush, 4)
	dispatcher := &dispatch.PushDispatcher{FCM: notificationRestartFCM{sent: deliveries}}
	runner := startNotificationAcceptanceConsumer(t, spec.service, func(ctx context.Context) error {
		return runModerationEventsConsumer(ctx, fixture.serverURL, fixture.tokens, dispatcher, nil, nil, profiles, nil)
	})
	encoded, err := proto.Marshal(spec.event)
	require.NoError(t, err)
	published, err := fixture.js.Publish(spec.subject, encoded)
	require.NoError(t, err)

	// The second handler stays blocked so neither a push nor ACK can race the
	// failure assertions. Its arrival within five seconds precedes AckWait and
	// therefore distinguishes explicit NAK from timeout-based redelivery.
	for attempt := 1; attempt <= 2; attempt++ {
		select {
		case got := <-profiles.attempts:
			require.Equal(t, accountID, got, "attempt %d resolves the same target account", attempt)
		case <-time.After(5 * time.Second):
			t.Fatalf("moderation resolver attempt %d was not reached before AckWait", attempt)
		}
	}
	require.Eventually(t, func() bool {
		info, infoErr := fixture.js.ConsumerInfo(spec.stream, durable)
		return infoErr == nil && info.NumAckPending == 1 && info.NumRedelivered == 1 &&
			info.AckFloor.Stream < published.Sequence && info.Delivered.Consumer >= 2
	}, 5*time.Second, 20*time.Millisecond, "resolver failure leaves the event unacknowledged and explicitly redelivered")
	require.Empty(t, deliveries, "transient resolution failure must not dispatch a push")

	profiles.unblock()
	select {
	case got := <-deliveries:
		require.Equal(t, spec.recipientID, got.profileID)
		require.Equal(t, spec.expected, got.payload)
	case <-time.After(5 * time.Second):
		t.Fatal("moderation retry did not deliver the recovered sanction notification")
	}
	require.Eventually(t, func() bool {
		info, infoErr := fixture.js.ConsumerInfo(spec.stream, durable)
		return infoErr == nil && info.NumPending == 0 && info.NumAckPending == 0 &&
			info.AckFloor.Stream >= published.Sequence
	}, 5*time.Second, 20*time.Millisecond, "successful retry ACKs the original event and drains backlog")
	err, joined := runner.stop()
	require.True(t, joined, "moderation consumer is joined before checking final deliveries")
	require.ErrorIs(t, err, context.Canceled)
	require.EqualValues(t, 2, profiles.calls.Load(), "one failed attempt and one recovered attempt")
	require.Empty(t, deliveries, "ACKed recovered event must not produce another push")
}
