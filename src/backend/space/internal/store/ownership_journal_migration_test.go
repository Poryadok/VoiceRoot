package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func ownershipJournalMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000008_ownership_journal."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func ownershipJournalDecisionMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000009_ownership_journal_decision."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func requireJournalSchema(t *testing.T, pool *pgxpool.Pool, present bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var table, index bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('ownership_journal') IS NOT NULL,to_regclass('ownership_journal_one_active_space') IS NOT NULL`).Scan(&table, &index))
	require.Equal(t, present, table)
	require.Equal(t, present, index)
}

func requireJournalDecisionSchema(t *testing.T, pool *pgxpool.Pool, present bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var receiptID, factors bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT
		EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='ownership_journal' AND column_name='auth_receipt_id'),
		EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='ownership_journal' AND column_name='auth_verified_factors')`).Scan(&receiptID, &factors))
	require.Equal(t, present, receiptID)
	require.Equal(t, present, factors)
}

func TestOwnershipJournalMigration_EmptyDownUpPreservesExistingSpace(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := st.Pool.Exec(ctx, ownershipJournalMigrationSQL(t, "down"))
	require.NoError(t, err)
	requireJournalSchema(t, st.Pool, false)
	row, err := st.GetSpace(ctx, binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, binding.ActorProfileID, row.OwnerProfileID)
	_, err = st.Pool.Exec(ctx, ownershipJournalMigrationSQL(t, "up"))
	require.NoError(t, err)
	decisionMigration, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000009_ownership_journal_decision.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(decisionMigration))
	require.NoError(t, err)
	requireJournalSchema(t, st.Pool, true)
	reservation, err := st.ReserveOwnership(ctx, binding)
	require.NoError(t, err)
	assertOwnershipReservation(t, reservation, binding)
}

func TestOwnershipJournalMigration_DownRefusesEveryEvidenceState(t *testing.T) {
	for _, state := range []string{"reserved", "completed", "aborted"} {
		t.Run(state, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			var expected []*OwnershipJournal
			for i := 0; i < 2; i++ {
				binding := seedOwnershipJournalBinding(t, st)
				_, err := st.ReserveOwnership(ctx, binding)
				require.NoError(t, err)
				// Only a fixture transition: this migration test does not implement or
				// claim proof/decision/finalization behavior of later journal cycles.
				_, err = st.Pool.Exec(ctx, `UPDATE ownership_journal SET state=$2 WHERE operation_id=$1`, binding.OperationID, state)
				require.NoError(t, err)
				row, err := st.LoadOwnership(ctx, binding.OperationID)
				require.NoError(t, err)
				expected = append(expected, row)
			}
			_, err := st.Pool.Exec(ctx, ownershipJournalMigrationSQL(t, "down"))
			require.Error(t, err, "even terminal evidence must forbid destructive migration rollback")
			var refusal *pgconn.PgError
			require.ErrorAs(t, err, &refusal)
			require.Equal(t, "P0001", refusal.Code)
			require.Contains(t, refusal.Message, "durable evidence")
			requireJournalSchema(t, st.Pool, true)
			var count int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM ownership_journal`).Scan(&count))
			require.Equal(t, len(expected), count)
			for _, want := range expected {
				got, err := st.LoadOwnership(ctx, want.Binding.OperationID)
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
		})
	}
}

// waitJournalDropLock observes a real PostgreSQL lock waiter. The timer only
// bounds observation; elapsed time never establishes that DOWN reached DROP.
func waitJournalDropLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pid int32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='relation' AND relation='ownership_journal'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`, pid).Scan(&waiting)
		require.NoError(t, err)
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("DOWN never waited for the journal relation lock: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestOwnershipJournalMigration_ConcurrentCommittedInsertCannotBeErasedByDown(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	downSQL := ownershipJournalMigrationSQL(t, "down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	insertTx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() {
		cleanupCtx, c := context.WithTimeout(context.Background(), time.Second)
		defer c()
		_ = insertTx.Rollback(cleanupCtx)
	}()
	// The insert is valid but invisible to DOWN's old preflight SELECT. Its
	// RowExclusiveLock blocks either the fixed upfront lock or the old DROP.
	encoded, digest, err := EncodeOwnershipBinding(binding)
	require.NoError(t, err)
	_, err = insertTx.Exec(ctx, `INSERT INTO ownership_journal(operation_id,protocol_version,space_id,account_id,actor_profile_id,new_owner_profile_id,session_epoch,proof_digest,binding_bytes,binding_hash,state,audit_id,event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'reserved',$11,$12)`, binding.OperationID, binding.ProtocolVersion, binding.SpaceID, binding.AccountID, binding.ActorProfileID, binding.NewOwnerProfileID, binding.SessionEpoch, binding.ProofDigest, encoded, digest[:], uuid.New(), uuid.New())
	require.NoError(t, err)
	downConn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	var pid int32
	require.NoError(t, downConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	result := make(chan error, 1)
	done := make(chan struct{})
	// Cleanup releases the blocker before waiting and never returns a connection
	// to the pool while the migration goroutine is still using it.
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
		_ = insertTx.Rollback(cleanupCtx)
		cleanupCancel()
		cancel()
		select {
		case <-done:
			downConn.Release()
		case <-time.After(3 * time.Second):
			t.Error("DOWN worker did not exit after cancellation")
		}
	}()
	go func() {
		defer close(done)
		_, err := downConn.Exec(ctx, downSQL)
		if err != nil {
			// An explicit BEGIN/LOCK/check/COMMIT migration remains aborted after a
			// refusal; release its locks before the observer tries to read the table.
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), time.Second)
			if _, rollbackErr := downConn.Exec(rollbackCtx, "ROLLBACK"); rollbackErr != nil {
				_ = downConn.Conn().Close(rollbackCtx)
			}
			rollbackCancel()
		}
		result <- err
	}()
	waitJournalDropLock(t, ctx, st.Pool, pid)
	require.NoError(t, insertTx.Commit(ctx), "the reservation commits while DOWN is already waiting for exclusive access")
	select {
	case downErr := <-result:
		require.Error(t, downErr, "DOWN must recheck evidence after acquiring exclusive access, not erase the committed insert")
		var refusal *pgconn.PgError
		require.ErrorAs(t, downErr, &refusal)
		require.Equal(t, "P0001", refusal.Code)
		require.Contains(t, refusal.Message, "durable evidence")
	case <-ctx.Done():
		t.Fatalf("DOWN did not finish after insert commit: %v", ctx.Err())
	}
	requireJournalSchema(t, st.Pool, true)
	row, err := st.LoadOwnership(ctx, binding.OperationID)
	require.NoError(t, err)
	assertOwnershipReservation(t, row, binding)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM ownership_journal`).Scan(&count))
	require.Equal(t, 1, count)
}

func TestOwnershipJournalDecisionMigration_EmptyDownUpPreservesReservationSchema(t *testing.T) {
	st := ownershipJournalStoreFixture(t)
	binding := seedOwnershipJournalBinding(t, st)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := st.Pool.Exec(ctx, ownershipJournalDecisionMigrationSQL(t, "down"))
	require.NoError(t, err)
	requireJournalSchema(t, st.Pool, true)
	requireJournalDecisionSchema(t, st.Pool, false)
	row, err := st.GetSpace(ctx, binding.SpaceID)
	require.NoError(t, err)
	require.Equal(t, binding.ActorProfileID, row.OwnerProfileID)

	_, err = st.Pool.Exec(ctx, ownershipJournalDecisionMigrationSQL(t, "up"))
	require.NoError(t, err)
	requireJournalDecisionSchema(t, st.Pool, true)
	_, err = st.ReserveOwnership(ctx, binding)
	require.NoError(t, err)
	confirmed, err := st.ConfirmOwnershipProof(ctx, binding, ownershipAuthReceiptFixture(binding))
	require.NoError(t, err)
	require.Equal(t, "proof_confirmed", confirmed.State)
}

func TestOwnershipJournalDecisionMigration_DownRefusesDecisionEvidenceWithoutLoss(t *testing.T) {
	for _, state := range []string{"proof_confirmed", "abort_decided"} {
		t.Run(state, func(t *testing.T) {
			st := ownershipJournalStoreFixture(t)
			binding := seedOwnershipJournalBinding(t, st)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := st.ReserveOwnership(ctx, binding)
			require.NoError(t, err)
			if state == "proof_confirmed" {
				_, err = st.ConfirmOwnershipProof(ctx, binding, ownershipAuthReceiptFixture(binding))
			} else {
				_, err = st.DecideOwnershipAbort(ctx, binding)
			}
			require.NoError(t, err)
			before, err := st.LoadOwnership(ctx, binding.OperationID)
			require.NoError(t, err)

			_, err = st.Pool.Exec(ctx, ownershipJournalDecisionMigrationSQL(t, "down"))
			require.Error(t, err)
			var refusal *pgconn.PgError
			require.ErrorAs(t, err, &refusal)
			require.Equal(t, "P0001", refusal.Code)
			require.Contains(t, refusal.Message, "decision evidence")
			requireJournalSchema(t, st.Pool, true)
			requireJournalDecisionSchema(t, st.Pool, true)
			after, loadErr := st.LoadOwnership(ctx, binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, before, after)
			assertOwnershipJournalHasNoSuccessEffects(t, st, binding)
		})
	}
}
