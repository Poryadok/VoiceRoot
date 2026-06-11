package mmevents

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoopPublisher(t *testing.T) {
	t.Parallel()
	var p NoopPublisher
	ctx := context.Background()
	require.NoError(t, p.PublishSearchStarted(ctx, "s", "p", "g", "m", "eu"))
	require.NoError(t, p.PublishSearchCancelled(ctx, "s", "p"))
	require.NoError(t, p.Close())
}

func TestNewJetStreamPublisher_EmptyURL(t *testing.T) {
	t.Parallel()
	_, err := NewJetStreamPublisher("")
	require.Error(t, err)
}
