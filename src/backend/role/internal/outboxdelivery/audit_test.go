package outboxdelivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type auditLeaseFixture struct {
	effects               []AuditEffect
	claimErr              error
	deliveryErr           error
	delivered, failed     []uuid.UUID
	ackToken, failedToken uuid.UUID
}

func (f *auditLeaseFixture) ClaimAuditEffects(context.Context, int, time.Duration) ([]AuditEffect, error) {
	return f.effects, f.claimErr
}
func (f *auditLeaseFixture) CompleteAuditEffect(_ context.Context, id, token uuid.UUID) (bool, error) {
	f.delivered = append(f.delivered, id)
	f.ackToken = token
	return f.deliveryErr == nil, f.deliveryErr
}
func (f *auditLeaseFixture) RetryAuditEffect(_ context.Context, id, token uuid.UUID) (bool, error) {
	f.failed = append(f.failed, id)
	f.failedToken = token
	return true, nil
}

type auditAppendFixture struct {
	payloads [][]byte
	err      error
}

func (f *auditAppendFixture) Append(_ context.Context, _ uuid.UUID, payload []byte) error {
	f.payloads = append(f.payloads, append([]byte(nil), payload...))
	return f.err
}

func newAuditEffect() AuditEffect {
	return AuditEffect{EventID: uuid.New(), LeaseToken: uuid.New(), RequestBytes: []byte("immutable-request")}
}

func TestAuditDispatcher_OnlyAcknowledgedAppendCompletesCurrentLease(t *testing.T) {
	effect := newAuditEffect()
	store := &auditLeaseFixture{effects: []AuditEffect{effect}}
	sink := &auditAppendFixture{}
	d := &AuditDispatcher{Store: store, Sink: sink}
	require.NoError(t, d.DispatchOnce(context.Background()))
	require.Equal(t, [][]byte{effect.RequestBytes}, sink.payloads)
	require.Equal(t, []uuid.UUID{effect.EventID}, store.delivered)
	require.Equal(t, effect.LeaseToken, store.ackToken)
	require.Empty(t, store.failed)
}

func TestAuditDispatcher_UnknownAppendRetainsSamePayloadForRetry(t *testing.T) {
	effect := newAuditEffect()
	store := &auditLeaseFixture{effects: []AuditEffect{effect}}
	sink := &auditAppendFixture{err: errors.New("append outcome unknown")}
	d := &AuditDispatcher{Store: store, Sink: sink}
	require.Error(t, d.DispatchOnce(context.Background()))
	require.Empty(t, store.delivered)
	require.Equal(t, []uuid.UUID{effect.EventID}, store.failed)
	require.Equal(t, effect.LeaseToken, store.failedToken)
	sink.err = nil
	// A later claim belongs to another lease; the immutable persisted fact stays.
	store.effects[0].LeaseToken = uuid.New()
	require.NoError(t, d.DispatchOnce(context.Background()))
	require.Equal(t, [][]byte{effect.RequestBytes, effect.RequestBytes}, sink.payloads)
	require.Equal(t, store.effects[0].LeaseToken, store.ackToken)
}

func TestAuditDispatcher_LostLocalAcknowledgementDoesNotRewriteRemoteFact(t *testing.T) {
	effect := newAuditEffect()
	store := &auditLeaseFixture{effects: []AuditEffect{effect}, deliveryErr: errors.New("commit response lost")}
	sink := &auditAppendFixture{}
	d := &AuditDispatcher{Store: store, Sink: sink}
	require.Error(t, d.DispatchOnce(context.Background()))
	// Leave lease recovery to storage: do not replace an ambiguous completion
	// with a second local state transition or mutate its stable payload.
	require.Empty(t, store.failed)
	store.deliveryErr = nil
	require.NoError(t, d.DispatchOnce(context.Background()))
	require.Equal(t, [][]byte{effect.RequestBytes, effect.RequestBytes}, sink.payloads)
}

func TestAuditDispatcher_ClaimFailureHasNoDeliveryEffect(t *testing.T) {
	claimErr := errors.New("database unavailable")
	store := &auditLeaseFixture{claimErr: claimErr}
	sink := &auditAppendFixture{}
	require.ErrorIs(t, (&AuditDispatcher{Store: store, Sink: sink}).DispatchOnce(context.Background()), claimErr)
	require.Empty(t, sink.payloads)
	require.Empty(t, store.delivered)
	require.Empty(t, store.failed)
}
