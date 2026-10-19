package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

// Apply the actual official34 upgrade with the pinned migration driver. The
// Space seed belongs to the explicit pre-native history; no version relabeling
// or production Creator/admission capability is manufactured by this fixture.
func joinAdmissionNativeFixture(t *testing.T) (*SpaceStore, *SpaceRow) {
	t.Helper()
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "goto", "34")
	st := &SpaceStore{Pool: database.Pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Historical join fixture", "", "public")
	require.NoError(t, err)
	database.Run(true, "up")
	return st, space
}

func TestJoinAttempt_DecisionCoordinatorPinsOneNativeIntent(t *testing.T) {
	ctx := context.Background()
	st, space := joinAdmissionNativeFixture(t)
	in := JoinAttemptInput{SpaceID: space.ID, AccountID: uuid.New(), ProfileID: uuid.New()}
	a, err := st.StartJoinAttempt(ctx, in)
	require.NoError(t, err)
	a, err = st.SubmitJoinAttempt(ctx, in, a.ID, nil)
	require.NoError(t, err)
	// Historical storage fixture only: production basis comes from the private
	// decision writer. No caller hash is a current owner/Role authorization.
	_, basis := seedJoinDecisionHistory(t, st, in, a)
	// Two independent callers both read the unpinned basis before native
	// Prepare can acquire the held Space fence. The loser must reload exactly
	// this basis's P rather than choose a new A or steal the winner's intent.
	gate, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = gate.Exec(ctx, `SELECT pg_advisory_lock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
	require.NoError(t, err)
	type result struct {
		intent *MemberAdmissionIntent
		err    error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() { i, err := st.PrepareJoinDecisionAdmission(ctx, basis); results <- result{i, err} }()
	}
	waitCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var waiting int
	for waitCtx.Err() == nil {
		err = st.Pool.QueryRow(waitCtx, `SELECT count(*) FROM pg_locks l JOIN pg_stat_activity a ON a.pid=l.pid WHERE l.locktype='advisory' AND NOT l.granted AND a.datname=current_database()`).Scan(&waiting)
		if err != nil || waiting >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	_, unlockErr := gate.Exec(ctx, `SELECT pg_advisory_unlock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
	gate.Release()
	require.NoError(t, err)
	require.NoError(t, unlockErr)
	require.GreaterOrEqual(t, waiting, 2, "both candidate operations must reach the actual Space lock")
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	i := first.intent
	require.Equal(t, i, second.intent)
	require.Equal(t, "PREPARING", i.State)
	require.Equal(t, in.AccountID, i.Input.AccountID)
	require.Equal(t, in.ProfileID, i.Input.ProfileID)
	require.Equal(t, &JoinAttemptReference{AttemptID: a.ID, EvidenceSHA256: a.EvidenceSHA256}, i.Input.JoinAttempt)
	require.Equal(t, basis, i.Input.DecisionBasisSHA256)
	again, err := st.PrepareJoinDecisionAdmission(ctx, basis)
	require.NoError(t, err)
	require.Equal(t, i, again)
	var state string
	var p *uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,publication_operation_id FROM space_join_attempts WHERE id=$1`, a.ID).Scan(&state, &p))
	require.Equal(t, "CLAIMED", state)
	require.Equal(t, &i.Input.OperationID, p)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_member_admission_intents WHERE join_attempt_id=$1`, a.ID).Scan(&count))
	require.Equal(t, 1, count)
	var members int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1 AND profile_id=$2`, space.ID, in.ProfileID).Scan(&members))
	require.Zero(t, members)
}
