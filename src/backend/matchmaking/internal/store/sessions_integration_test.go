package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSessionStore_CreateAndCancel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &GameStore{Pool: pool}
	g, err := gameStore.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)
	require.NotEmpty(t, g.Games)

	sessions := &SessionStore{Pool: pool}
	profileID := uuid.New()
	timeout := time.Now().UTC().Add(30 * time.Minute)
	sess, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileID,
		GameID:    g.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: timeout,
	})
	require.NoError(t, err)
	require.Equal(t, SessionStatusSearching, sess.Status)

	_, err = sessions.Create(ctx, CreateSessionParams{
		ProfileID: profileID,
		GameID:    g.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: timeout,
	})
	require.ErrorIs(t, err, ErrActiveSearchExists)

	cancelled, err := sessions.Cancel(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusCancelled, cancelled.Status)
}

func TestSessionStore_NudgeAndExpire(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)

	gameStore := &GameStore{Pool: pool}
	g, err := gameStore.List(ctx, ListGamesParams{PageSize: 1, Status: StatusActive})
	require.NoError(t, err)

	sessions := &SessionStore{Pool: pool}
	now := time.Now().UTC()
	sess, err := sessions.Create(ctx, CreateSessionParams{
		ProfileID: uuid.New(),
		GameID:    g.Games[0].ID,
		Mode:      "5v5 Ranked",
		Criteria:  `{"region":"eu","self":{"role":"Carry","rank":"Herald"}}`,
		TimeoutAt: now.Add(-time.Minute),
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET created_at = $2 WHERE id = $1`, sess.ID, now.Add(-20*time.Minute))
	require.NoError(t, err)

	needNudge, err := sessions.ListSearchingNeedingNudge(ctx, now.Add(-15*time.Minute), 10)
	require.NoError(t, err)
	require.Len(t, needNudge, 1)

	nudged, err := sessions.MarkNudged(ctx, sess.ID)
	require.NoError(t, err)
	require.NotNil(t, nudged.NudgedAt)

	_, err = sessions.MarkNudged(ctx, sess.ID)
	require.ErrorIs(t, err, ErrSessionNotSearchable)

	expired, err := sessions.ListSearchingExpired(ctx, now, 10)
	require.NoError(t, err)
	require.Len(t, expired, 1)

	timedOut, err := sessions.ExpireSearching(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, SessionStatusTimeout, timedOut.Status)
}
