package ordinarycall

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type relayStore struct {
	item                OutboxItem
	released, delivered int
}

func (s *relayStore) ClaimOutbox(context.Context, time.Duration) (*OutboxItem, error) {
	v := s.item
	return &v, nil
}
func (s *relayStore) MarkDelivered(ctx context.Context, _ OutboxItem) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.delivered++
	return nil
}
func (s *relayStore) ReleaseOutbox(ctx context.Context, _ OutboxItem, _ time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.released++
	return nil
}

type relayPublisher func(context.Context, uuid.UUID, string, []byte) error

func (f relayPublisher) PublishOrdinaryEvent(ctx context.Context, id uuid.UUID, subject string, b []byte) error {
	return f(ctx, id, subject, b)
}

func TestOutboxUnknownPubAckKeepsImmutableIntentAndUsesFreshRetryContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store := &relayStore{item: OutboxItem{EncodedEvent: EncodedEvent{ID: uuid.New(), Subject: "voice.call_ended", Payload: []byte("saved protobuf")}, LeaseToken: uuid.New()}}
	original := store.item
	relay := &OutboxRelay{Store: store, Publisher: relayPublisher(func(_ context.Context, id uuid.UUID, subject string, b []byte) error {
		require.Equal(t, original.ID, id)
		require.Equal(t, original.Subject, subject)
		require.Equal(t, original.Payload, b)
		cancel()
		return context.DeadlineExceeded
	})}
	claimed, err := relay.DispatchOnce(ctx)
	require.True(t, claimed)
	require.Error(t, err)
	require.Equal(t, 1, store.released)
	require.Zero(t, store.delivered)
	relay.Publisher = relayPublisher(func(_ context.Context, id uuid.UUID, subject string, b []byte) error {
		require.Equal(t, original.ID, id)
		require.Equal(t, original.Subject, subject)
		require.Equal(t, original.Payload, b)
		return nil
	})
	claimed, err = relay.DispatchOnce(context.Background())
	require.True(t, claimed)
	require.NoError(t, err)
	require.Equal(t, 1, store.delivered)
	require.Equal(t, original, store.item)
}
