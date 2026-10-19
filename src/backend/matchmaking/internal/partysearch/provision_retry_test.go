package partysearch

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

func TestProvisionProgressClockRequiresNewDurableEvidence(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	stale := func() {
		t.Helper()
		_, err := pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_last_progress_at=clock_timestamp()-interval '20 minutes' WHERE match_id=$1`, intent.MatchID)
		require.NoError(t, err)
	}
	stale()
	_, ok, err := st.ClaimMatchSquadProvisioningOperation(ctx, intent.MatchID, intent.OperationID, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	attention, err := st.MatchSquadProvisioningAttention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, attention.Stalled, "claim/retry is not provider progress or cleanup completion")
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	attention, err = st.MatchSquadProvisioningAttention(ctx)
	require.NoError(t, err)
	require.Zero(t, attention.Stalled, "first durable issued obligation is progress")
	stale()
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	attention, err = st.MatchSquadProvisioningAttention(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, attention.Stalled, "repeat authorization must not hide stalled creation")
	require.NoError(t, st.AbandonMatch(ctx, intent.MatchID))
	attention, err = st.MatchSquadProvisioningAttention(ctx)
	require.NoError(t, err)
	require.Zero(t, attention.Stalled, "first monotone abandonment creates durable cleanup obligations")
}

func TestProvisionRetryClaimFencesStaleSchedulingAndPreservesIssuedObligation(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := dispatchFixture(t)
	candidate, proof := dispatchProof(t, st, intent.MatchID, p)
	_, err := st.SaveValidatedMatchSquadProvisioningIntent(ctx, candidate, proof, intent)
	require.NoError(t, err)
	first, ok, err := st.ClaimMatchSquadProvisioningOperation(ctx, intent.MatchID, intent.OperationID, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	require.EqualValues(t, 1, first.Generation)
	require.Equal(t, 1, first.AttemptCount)
	_, ok, err = st.ClaimMatchSquadProvisioningOperation(ctx, intent.MatchID, intent.OperationID, time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "a live claim cannot be borrowed by another worker")
	candidate, proof = dispatchProof(t, st, intent.MatchID, p)
	_, err = st.AuthorizeMatchSquadProviderDispatch(ctx, candidate, proof, intent.OperationID, "chat")
	require.NoError(t, err)
	require.NoError(t, st.AbandonMatch(ctx, intent.MatchID))
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_claim_until=clock_timestamp()-interval '1 second' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	second, ok, err := st.ClaimMatchSquadProvisioningOperation(ctx, intent.MatchID, intent.OperationID, time.Minute)
	require.NoError(t, err)
	require.True(t, ok, "compensating unknown creation must remain claimable")
	require.Equal(t, first.Generation+1, second.Generation)
	require.ErrorIs(t, st.ScheduleMatchSquadProvisioningRetry(ctx, first, time.Second, "unavailable"), store.ErrMatchSquadConflict)
	require.ErrorIs(t, st.ScheduleMatchSquadProvisioningRetry(ctx, second, time.Second, "private provider error or identifier"), store.ErrMatchSquadConflict)
	require.NoError(t, st.ScheduleMatchSquadProvisioningRetry(ctx, second, time.Hour, "unavailable"))
	var delay float64
	var marker, disposition, class string
	var released bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM provision_next_attempt_at-clock_timestamp()),chat_dispatch_state,provision_disposition,provision_last_error_class,provision_claim_until IS NULL
		FROM matchmaking_match_squad_operations WHERE match_id=$1`, intent.MatchID).Scan(&delay, &marker, &disposition, &class, &released))
	require.InDelta(t, 300, delay, 2)
	require.Equal(t, "MAY_HAVE_CREATED", marker)
	require.Equal(t, "ABANDONED", disposition)
	require.Equal(t, "unavailable", class)
	require.True(t, released)
	_, ok, err = st.ClaimMatchSquadProvisioningOperation(ctx, intent.MatchID, intent.OperationID, time.Minute)
	require.NoError(t, err)
	require.False(t, ok, "persisted backoff must survive another request or process restart")
	_, err = pool.Exec(ctx, `UPDATE matchmaking_match_squad_operations SET provision_next_attempt_at=clock_timestamp()-interval '1 second' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	type result struct {
		claim store.MatchSquadProvisioningClaim
		ok    bool
		err   error
	}
	results := make(chan result, 8)
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			claim, ok, err := st.ClaimMatchSquadProvisioningOperation(ctx, intent.MatchID, intent.OperationID, time.Minute)
			results <- result{claim, ok, err}
		}()
	}
	workers.Wait()
	close(results)
	winners := 0
	var winner store.MatchSquadProvisioningClaim
	for result := range results {
		require.NoError(t, result.err)
		if result.ok {
			winners++
			winner = result.claim
		}
	}
	require.Equal(t, 1, winners, "concurrent processes cannot both receive one live claim")
	require.Equal(t, second.Generation+1, winner.Generation)
	require.ErrorIs(t, st.ScheduleMatchSquadProvisioningRetry(ctx, second, time.Second, "unavailable"), store.ErrMatchSquadConflict)
	require.NoError(t, st.ReleaseMatchSquadProvisioningClaim(ctx, winner))
	require.ErrorIs(t, st.ReleaseMatchSquadProvisioningClaim(ctx, winner), store.ErrMatchSquadConflict)
}
