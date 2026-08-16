package queue

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRedisQueue_SpaceScopedKeysIsolated(t *testing.T) {
	t.Parallel()
	q, cleanup := newTestQueue(t)
	t.Cleanup(cleanup)
	ctx := context.Background()
	gameID := uuid.New()
	spaceID := uuid.New()
	mode := "Duo"
	region := "eu"
	spaceSess := uuid.New()
	globalSess := uuid.New()
	now := time.Now().UTC()

	require.NoError(t, q.EnqueueScoped(ctx, &spaceID, gameID, mode, region, spaceSess, now))
	require.NoError(t, q.Enqueue(ctx, gameID, mode, region, globalSess, now))

	spaceDepth, err := q.QueueDepthScoped(ctx, &spaceID, gameID, mode, region)
	require.NoError(t, err)
	require.Equal(t, int64(1), spaceDepth)

	globalDepth, err := q.QueueDepth(ctx, gameID, mode, region)
	require.NoError(t, err)
	require.Equal(t, int64(1), globalDepth)

	spaceIDs, err := q.ListSessionIDsScoped(ctx, &spaceID, gameID, mode, region, 0)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{spaceSess}, spaceIDs)

	globalIDs, err := q.ListSessionIDs(ctx, gameID, mode, region, 0)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{globalSess}, globalIDs)
}
