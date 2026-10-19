package rosterruntime

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/matchmaking/internal/mmevents"
)

type capturedCancellationSpy struct {
	mmevents.NoopPublisher
	id  uuid.UUID
	raw []byte
}

func (p *capturedCancellationSpy) PublishCapturedCancellation(_ context.Context, id uuid.UUID, raw []byte) error {
	p.id = id
	p.raw = append([]byte(nil), raw...)
	return nil
}
func TestNativeCancellationPreparationNeverUsesRosterOnlyOrNoopPublisher(t *testing.T) {
	_, err := nativeCancellationEvents(mmevents.NoopPublisher{})
	require.ErrorIs(t, err, ErrUnavailable)
	var missing *capturedCancellationSpy
	_, err = nativeCancellationEvents(missing)
	require.ErrorIs(t, err, ErrUnavailable)
	actual := &capturedCancellationSpy{}
	prepared, err := nativeCancellationEvents(actual)
	require.NoError(t, err)
	publisher, ok := prepared.(cancellationPublisher)
	require.True(t, ok)
	id := uuid.New()
	raw := []byte("frozen terminal fixture")
	require.NoError(t, publisher.PublishRosterCancellation(context.Background(), id, raw))
	require.Equal(t, id, actual.id)
	require.Equal(t, raw, actual.raw)
}
