package store_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/bot/internal/store"
)

func TestBotDeletionWorkerClaimsDueRootsAndFencesRetryRelease(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL worker claim scheduling")
	}
	st := startBotStore(t)
	ctx := t.Context()
	var roots []*store.BotDeletion
	for range 2 {
		owner := uuid.New()
		bot, _, err := st.CreateBot(ctx, owner, "Worker", "", `[]`, uuid.New())
		require.NoError(t, err)
		_, err = st.PrepareInstallation(ctx, bot.ID, uuid.New(), uuid.New(), nil)
		require.NoError(t, err)
		root, err := st.BeginBotDeletion(ctx, bot.ID, owner)
		require.NoError(t, err)
		roots = append(roots, root)
	}
	locked, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer locked.Rollback(context.Background())
	_, err = locked.Exec(ctx, `SELECT operation_id FROM bot_deletion_operations WHERE operation_id=$1 FOR UPDATE`, roots[0].OperationID)
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	claim, err := st.ClaimNextBotDeletion(bounded)
	require.NoError(t, err)
	require.Equal(t, roots[1].OperationID, claim.OperationID, "skip another writer's locked root")
	_, err = st.ClaimNextBotDeletion(bounded)
	require.ErrorIs(t, err, store.ErrDeletionClaimLost)
	require.NoError(t, locked.Rollback(ctx))
	type outcome struct {
		root *store.BotDeletion
		err  error
	}
	start := make(chan struct{})
	done := make(chan outcome, 4)
	for range 4 {
		go func() { <-start; root, err := st.ClaimNextBotDeletion(ctx); done <- outcome{root, err} }()
	}
	close(start)
	var winner *store.BotDeletion
	for range 4 {
		result := <-done
		if result.err == nil {
			require.Nil(t, winner)
			winner = result.root
		} else {
			require.ErrorIs(t, result.err, store.ErrDeletionClaimLost)
		}
	}
	require.NotNil(t, winner)
	require.Equal(t, roots[0].OperationID, winner.OperationID)
	require.NoError(t, st.RetryBotDeletion(ctx, winner))
	_, err = st.ClaimNextBotDeletion(ctx)
	require.ErrorIs(t, err, store.ErrDeletionClaimLost, "retry delay is respected")
	_, err = st.Pool.Exec(ctx, `UPDATE bot_deletion_operations SET next_attempt_at=clock_timestamp()-interval '1 second' WHERE operation_id=$1`, winner.OperationID)
	require.NoError(t, err)
	next, err := st.ClaimNextBotDeletion(ctx)
	require.NoError(t, err)
	require.Greater(t, next.ClaimGeneration, winner.ClaimGeneration)
	require.ErrorIs(t, st.RetryBotDeletion(ctx, winner), store.ErrDeletionClaimLost, "stale release cannot clear successor's claim")
	var state string
	var generation int64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,claim_generation FROM bot_deletion_operations WHERE operation_id=$1`, next.OperationID).Scan(&state, &generation))
	require.Equal(t, "RUNNING", state)
	require.Equal(t, next.ClaimGeneration, generation)
}
