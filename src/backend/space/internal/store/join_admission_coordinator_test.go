package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

// Own DSN-aware fixture applies the canonical native Creator DDL and real Q32.
// It does not change the producer's Testcontainers-only fixture or seed an
// authenticated caller. signedNativeCreator still verifies its separate purpose.
func joinAdmissionNativeFixture(t *testing.T) *SpaceStore {
	t.Helper()
	ctx := context.Background()
	pool := startJoinAttemptPostgres(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	for _, name := range []string{"000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql", "000024_authority_revision.up.sql", "000030_entry_policy.up.sql", "000032_join_request_groups.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	st := &SpaceStore{Pool: pool}
	applyMemberAuthorityIfPresent(t, st)
	_, err := pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(33,false)`)
	require.NoError(t, err)
	return st
}

func TestJoinAttempt_DecisionCoordinatorPinsOneNativeIntent(t *testing.T) {
	ctx := context.Background()
	st := joinAdmissionNativeFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	create, _, _ := signedNativeCreator(t, st, &spacev1.CreateSpaceRequest{Name: "Decision coordinator", Visibility: "public"})
	space, err := create()
	require.NoError(t, err)
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
