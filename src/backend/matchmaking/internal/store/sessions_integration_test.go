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
