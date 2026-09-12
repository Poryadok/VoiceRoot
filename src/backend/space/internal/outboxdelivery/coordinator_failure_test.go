package outboxdelivery

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

type retryAwareStore struct {
	*fakeDeliveryStore
	retryCalls  []deliveryMark
	retryResult bool
	retryErr    error
}

func newRetryAwareStore() *retryAwareStore {
	return &retryAwareStore{fakeDeliveryStore: newFakeDeliveryStore(), retryResult: true}
}

func (s *retryAwareStore) MarkFailed(_ context.Context, eventID, leaseToken uuid.UUID) (bool, error) {
	s.retryCalls = append(s.retryCalls, deliveryMark{eventID: eventID, leaseToken: leaseToken})
	return s.retryResult, s.retryErr
}

func TestOwnershipOutboxDelivery_KnownPublishOrAckFailureReleasesLeaseForDatabaseRetry(t *testing.T) {
	publishErr := errors.New("publish unavailable")
	tests := []struct {
		name    string
		outcome publishOutcome
	}{
		{name: "publish error", outcome: publishOutcome{err: publishErr, record: true}},
		{name: "missing acknowledgement", outcome: publishOutcome{ack: nil, record: true}},
		{name: "wrong stream", outcome: publishOutcome{ack: &nats.PubAck{Stream: "other_events", Sequence: 1}, record: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newRetryAwareStore()
			transport := &scriptedTransport{outcomes: []publishOutcome{tt.outcome}}

			err := NewCoordinator(store, transport).DispatchOnce(context.Background())
			require.Error(t, err)
			require.False(t, store.delivered)
			require.Empty(t, store.markCalls)
			require.Equal(t, []deliveryMark{{eventID: testEventID, leaseToken: testLease}}, store.retryCalls,
				"known failure must release to the store's DB-time retry schedule instead of waiting for lease expiry")
		})
	}
}

func TestOwnershipOutboxDelivery_FailureReleaseMustMatchCurrentLeaseAndPersist(t *testing.T) {
	retryErr := errors.New("retry state unavailable")
	tests := []struct {
		name        string
		retryResult bool
		retryErr    error
	}{
		{name: "stale token", retryResult: false},
		{name: "database failure", retryResult: false, retryErr: retryErr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newRetryAwareStore()
			store.retryResult = tt.retryResult
			store.retryErr = tt.retryErr
			transport := &scriptedTransport{outcomes: []publishOutcome{{err: errors.New("publish failed")}}}

			err := NewCoordinator(store, transport).DispatchOnce(context.Background())
			require.Error(t, err)
			if tt.retryErr != nil {
				require.ErrorIs(t, err, tt.retryErr)
			}
			require.False(t, store.delivered)
			require.Empty(t, store.markCalls)
			require.Len(t, store.retryCalls, 1)
		})
	}
}

func TestOwnershipOutboxDelivery_ExactDeterministicBytesAndOnlyStableMessageIDHeader(t *testing.T) {
	store := newFakeDeliveryStore()
	transport := &scriptedTransport{outcomes: []publishOutcome{{ack: correctAck(), record: true}}}

	require.NoError(t, NewCoordinator(store, transport).DispatchOnce(context.Background()))
	require.Len(t, transport.msgs, 1)

	wantEnvelope := &eventsv1.ChatStreamEvent{
		EventId:    testEventID.String(),
		OccurredAt: timestamppb.New(testCreatedAt),
		Payload: &eventsv1.ChatStreamEvent_SpaceUpdated{
			SpaceUpdated: &eventsv1.SpaceUpdated{SpaceId: testSpaceID.String()},
		},
	}
	wantBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(wantEnvelope)
	require.NoError(t, err)
	require.Equal(t, wantBytes, transport.msgs[0].Data)
	require.Equal(t, nats.Header{nats.MsgIdHdr: []string{testEventID.String()}}, transport.msgs[0].Header)
}

type blockingDispatcher struct {
	started          chan struct{}
	cancellationSeen chan struct{}
	release          chan struct{}
	active           atomic.Bool
}

func (d *blockingDispatcher) DispatchOnce(ctx context.Context) error {
	d.active.Store(true)
	close(d.started)
	<-ctx.Done()
	close(d.cancellationSeen)
	<-d.release
	d.active.Store(false)
	return ctx.Err()
}

func TestWorker_CancellationWaitsForInFlightDispatchToExit(t *testing.T) {
	dispatcher := &blockingDispatcher{
		started:          make(chan struct{}),
		cancellationSeen: make(chan struct{}),
		release:          make(chan struct{}),
	}
	worker := NewWorker(dispatcher, time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(ctx) }()

	select {
	case <-dispatcher.started:
	case <-time.After(time.Second):
		t.Fatal("worker did not start its immediate dispatch")
	}
	cancel()
	select {
	case <-dispatcher.cancellationSeen:
	case <-time.After(time.Second):
		t.Fatal("worker did not propagate cancellation to the in-flight dispatch")
	}
	select {
	case err := <-done:
		t.Fatalf("worker returned while its mutation was still active: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	require.True(t, dispatcher.active.Load())

	close(dispatcher.release)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("worker did not finish after the in-flight dispatch exited")
	}
	require.False(t, dispatcher.active.Load())
}
