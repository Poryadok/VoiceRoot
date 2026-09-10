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

func ownershipJournalCommitMigrationSQL(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000010_ownership_journal_commit."+direction+".sql"))
	require.NoError(t, err)
	return string(raw)
}

func requireOwnershipCommitSchema(t *testing.T, pool *pgxpool.Pool, present bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var columns, outbox bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT
		(SELECT count(*) = 8 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='ownership_journal' AND column_name IN (
			'role_receipt_bytes','role_receipt_hash','role_intent_bytes','role_intent_hash',
			'pending_audit_action','pending_audit_target_type','pending_audit_target_id','pending_audit_details'
		)),
		to_regclass('ownership_outbox') IS NOT NULL`).Scan(&columns, &outbox))
	require.Equal(t, present, columns)
	require.Equal(t, present, outbox)
}

func execOwnershipCommitDown(t *testing.T, ctx context.Context, pool *pgxpool.Pool) error {
	t.Helper()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, ownershipJournalCommitMigrationSQL(t, "down"))
	if err != nil {
		rollbackCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, rollbackErr := conn.Exec(rollbackCtx, "ROLLBACK"); rollbackErr != nil {
			_ = conn.Conn().Close(rollbackCtx)
		}
	}
	return err
}

func TestOwnershipJournalCommitMigration_EmptyDownUpPreservesPriorReceipt(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	binding := seedOwnershipProofConfirmed(t, st)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	before, err := st.LoadOwnership(ctx, binding.OperationID)
	require.NoError(t, err)

	require.NoError(t, execOwnershipCommitDown(t, ctx, st.Pool))
	requireOwnershipCommitSchema(t, st.Pool, false)
	var state string
	var receiptID uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state,auth_receipt_id FROM ownership_journal WHERE operation_id=$1`, binding.OperationID).Scan(&state, &receiptID))
	require.Equal(t, "proof_confirmed", state)
	require.Equal(t, before.AuthReceipt.ReceiptID, receiptID)

	_, err = st.Pool.Exec(ctx, ownershipJournalCommitMigrationSQL(t, "up"))
	require.NoError(t, err)
	requireOwnershipCommitSchema(t, st.Pool, true)
	after, err := st.LoadOwnership(ctx, binding.OperationID)
	require.NoError(t, err)
	require.Equal(t, before, after)
	assertNoOwnershipCommitEffects(t, st, binding, binding.ActorProfileID)
}

func TestOwnershipJournalCommitMigration_DownRefusesPreparedDecisionAndTerminalEvidence(t *testing.T) {
	for _, state := range []string{"prepared", "abort_decided", "aborted", "commit_decided", "completed"} {
		t.Run(state, func(t *testing.T) {
			st := ownershipJournalCommitStoreFixture(t)
			binding := seedOwnershipProofConfirmed(t, st)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, err := st.MarkOwnershipPrepared(ctx, binding, ownershipRolePreparedReceiptFixture(binding))
			require.NoError(t, err)
			expectedOwner := binding.ActorProfileID
			wantOutbox := 0
			switch state {
			case "abort_decided", "aborted":
				_, err = st.DecideOwnershipAbort(ctx, binding)
			case "commit_decided", "completed":
				_, err = st.DecideOwnershipCommit(ctx, binding)
				expectedOwner = binding.NewOwnerProfileID
				wantOutbox = 1
			}
			require.NoError(t, err)
			if state == "aborted" {
				_, err = st.Pool.Exec(ctx, `UPDATE ownership_journal SET state='aborted' WHERE operation_id=$1`, binding.OperationID)
				require.NoError(t, err)
			}
			if state == "completed" {
				_, err = st.Pool.Exec(ctx, `UPDATE ownership_journal SET state='completed' WHERE operation_id=$1`, binding.OperationID)
				require.NoError(t, err)
				_, err = st.Pool.Exec(ctx, `UPDATE ownership_outbox SET ready=true WHERE operation_id=$1`, binding.OperationID)
				require.NoError(t, err)
			}
			before, err := st.LoadOwnership(ctx, binding.OperationID)
			require.NoError(t, err)

			err = execOwnershipCommitDown(t, ctx, st.Pool)
			require.Error(t, err)
			var refusal *pgconn.PgError
			require.ErrorAs(t, err, &refusal)
			require.Equal(t, "P0001", refusal.Code)
			require.Contains(t, refusal.Message, "commit evidence")
			requireOwnershipCommitSchema(t, st.Pool, true)
			after, loadErr := st.LoadOwnership(ctx, binding.OperationID)
			require.NoError(t, loadErr)
			require.Equal(t, before, after)
			space, getErr := st.GetSpace(ctx, binding.SpaceID)
			require.NoError(t, getErr)
			require.Equal(t, expectedOwner, space.OwnerProfileID)
			var outboxCount int
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM ownership_outbox WHERE operation_id=$1`, binding.OperationID).Scan(&outboxCount))
			require.Equal(t, wantOutbox, outboxCount)
			page, pageErr := st.ListAuditLogPage(ctx, binding.SpaceID, "", 10)
			require.NoError(t, pageErr)
			require.Empty(t, page.Rows)
		})
	}
}

func TestOwnershipJournalCommitMigration_OutboxEventAndOperationKeysAreDatabaseUnique(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	eventID, operationID := uuid.New(), uuid.New()
	spaceID, previousOwner, newOwner := uuid.New(), uuid.New(), uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO ownership_outbox(event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,ready) VALUES($1,$2,$3,$4,$5,'space.updated',false)`, eventID, operationID, spaceID, previousOwner, newOwner)
	require.NoError(t, err)
	for _, duplicate := range []struct {
		name           string
		eventID        uuid.UUID
		operationID    uuid.UUID
		wantConstraint string
	}{
		{"operation", uuid.New(), operationID, "ownership_outbox_operation_id_key"},
		{"event", eventID, uuid.New(), "ownership_outbox_pkey"},
	} {
		t.Run(duplicate.name, func(t *testing.T) {
			_, insertErr := st.Pool.Exec(ctx, `INSERT INTO ownership_outbox(event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,ready) VALUES($1,$2,$3,$4,$5,'space.updated',false)`, duplicate.eventID, duplicate.operationID, spaceID, previousOwner, newOwner)
			var constraintErr *pgconn.PgError
			require.ErrorAs(t, insertErr, &constraintErr)
			require.Equal(t, "23505", constraintErr.Code)
			require.Equal(t, duplicate.wantConstraint, constraintErr.ConstraintName)
		})
	}
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM ownership_outbox`).Scan(&count))
	require.Equal(t, 1, count)
}

func waitOwnershipCommitDownLockOrder(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pid int32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var journalGranted, outboxWaiting bool
		err := pool.QueryRow(ctx, `SELECT
			EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='relation' AND relation='ownership_journal'::regclass AND mode='AccessExclusiveLock' AND granted),
			EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='relation' AND relation='ownership_outbox'::regclass AND mode='AccessExclusiveLock' AND NOT granted)`, pid).Scan(&journalGranted, &outboxWaiting)
		require.NoError(t, err)
		if journalGranted && outboxWaiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("DOWN did not acquire journal before waiting for outbox: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestOwnershipJournalCommitMigration_DownLocksJournalThenOutboxAndPreservesConcurrentDeliveredEvidence(t *testing.T) {
	st := ownershipJournalCommitStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	downSQL := ownershipJournalCommitMigrationSQL(t, "down")
	insertTx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
		defer cleanupCancel()
		_ = insertTx.Rollback(cleanupCtx)
	}()
	eventID, operationID := uuid.New(), uuid.New()
	_, err = insertTx.Exec(ctx, `INSERT INTO ownership_outbox(event_id,operation_id,space_id,previous_owner_profile_id,new_owner_profile_id,event_type,ready) VALUES($1,$2,$3,$4,$5,'space.updated',true)`, eventID, operationID, uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)

	downConn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	var pid int32
	require.NoError(t, downConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	result := make(chan error, 1)
	done := make(chan struct{})
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Second)
		_ = insertTx.Rollback(cleanupCtx)
		cleanupCancel()
		cancel()
		select {
		case <-done:
			downConn.Release()
		case <-time.After(3 * time.Second):
			t.Error("commit migration DOWN worker did not exit after cancellation")
		}
	}()
	go func() {
		defer close(done)
		_, downErr := downConn.Exec(ctx, downSQL)
		if downErr != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), time.Second)
			if _, rollbackErr := downConn.Exec(rollbackCtx, "ROLLBACK"); rollbackErr != nil {
				_ = downConn.Conn().Close(rollbackCtx)
			}
			rollbackCancel()
		}
		result <- downErr
	}()
	waitOwnershipCommitDownLockOrder(t, ctx, st.Pool, pid)
	require.NoError(t, insertTx.Commit(ctx))
	select {
	case downErr := <-result:
		require.Error(t, downErr)
		var refusal *pgconn.PgError
		require.ErrorAs(t, downErr, &refusal)
		require.Equal(t, "P0001", refusal.Code)
		require.Contains(t, refusal.Message, "commit evidence")
	case <-ctx.Done():
		t.Fatalf("commit migration DOWN did not finish after outbox commit: %v", ctx.Err())
	}
	requireOwnershipCommitSchema(t, st.Pool, true)
	var ready bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT ready FROM ownership_outbox WHERE event_id=$1 AND operation_id=$2`, eventID, operationID).Scan(&ready))
	require.True(t, ready, "delivered or otherwise terminal outbox evidence must survive rollback refusal")
}
