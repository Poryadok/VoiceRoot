package rosterruntime

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/mmevents"
)

type fixturePublisher struct{ mmevents.NoopPublisher }

func (*fixturePublisher) PublishRosterCancellation(context.Context, uuid.UUID, []byte) error {
	return nil
}

func TestPrepareRejectsMissingDependenciesBeforeBinding(t *testing.T) {
	var typedNil *fixturePublisher
	for _, publisher := range []mmevents.Publisher{nil, mmevents.NoopPublisher{}, typedNil} {
		prepared, err := Prepare(context.Background(), &pgxpool.Pool{}, "nats://fixture", publisher, prometheus.NewRegistry(), nil)
		require.ErrorIs(t, err, ErrUnavailable)
		require.Nil(t, prepared)
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	prepared, err := Prepare(canceled, &pgxpool.Pool{}, "nats://fixture", &fixturePublisher{}, prometheus.NewRegistry(), nil)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Nil(t, prepared)
	prepared, err = Prepare(context.Background(), &pgxpool.Pool{}, "  ", &fixturePublisher{}, prometheus.NewRegistry(), nil)
	require.ErrorIs(t, err, ErrUnavailable)
	require.Nil(t, prepared)
}
