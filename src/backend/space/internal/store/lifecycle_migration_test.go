package store

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
)

func applyLifecycleMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, direction string) {
	t.Helper()
	raw, err := os.ReadFile(lifecycleMigrationPath(t, direction))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(raw))
	require.NoError(t, err)
}

func requireLifecycleSchema(t *testing.T, pool *pgxpool.Pool, present bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var operations, aggregates, participants, outbox, tombstones bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT
		to_regclass('space_lifecycle_operations') IS NOT NULL,
		to_regclass('space_lifecycle_aggregates') IS NOT NULL,
		to_regclass('space_lifecycle_participants') IS NOT NULL,
		to_regclass('space_lifecycle_outbox') IS NOT NULL,
		to_regclass('space_deletion_tombstones') IS NOT NULL`).Scan(&operations, &aggregates, &participants, &outbox, &tombstones))
	require.Equal(t, []bool{present, present, present, present, present}, []bool{operations, aggregates, participants, outbox, tombstones})
}

func TestLifecycleMigration_000013IsAdditiveAfterCurrent000012(t *testing.T) {
	up := lifecycleMigrationPath(t, "up")
	down := lifecycleMigrationPath(t, "down")
	require.NotEqual(t, up, down)

	if testing.Short() {
		return
	}
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	applyLifecycleMigration(t, ctx, pool, "up")
	requireLifecycleSchema(t, pool, true)

	for table, columns := range map[string][]string{
		"space_lifecycle_operations": {
			"account_id", "actor_profile_id", "session_epoch", "operation_id", "space_id", "method", "request_sha256", "confirmation_name_sha256", "proof_digest_sha256", "state", "auth_receipt_bytes", "auth_receipt_sha256", "outcome_bytes", "outcome_sha256", "completed_at",
		},
		"space_lifecycle_aggregates": {
			"space_id", "deletion_operation_id", "phase", "generation", "manifest_id", "manifest_sha256", "manifest_item_count", "scheduled_at", "purge_after", "purge_decided_at", "completed_at", "local_purge_completed",
		},
		"space_lifecycle_participants": {
			"space_id", "deletion_operation_id", "generation", "participant_id", "request_kind", "progress", "request_bytes", "request_sha256", "receipt_bytes", "receipt_sha256", "manifest_id", "manifest_sha256", "manifest_item_count", "completed_at",
		},
		"space_lifecycle_outbox": {
			"event_id", "space_id", "deletion_operation_id", "generation", "event_type", "state", "occurred_at", "event_bytes", "event_sha256", "created_at", "delivered_at",
		},
		"space_deletion_tombstones": {
			"space_id", "owner_account_hmac", "actor_account_hmac", "key_version", "reason", "scheduled_at", "purge_after", "purge_decided_at", "purged_at", "retain_until",
		},
	} {
		for _, column := range columns {
			var exists bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name=$1 AND column_name=$2)`, table, column).Scan(&exists))
			require.Truef(t, exists, "%s.%s is required durable R23 evidence", table, column)
		}
	}
	var tombstoneColumns, forbidden int
	var hasForeignKey bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='space_deletion_tombstones'`).Scan(&tombstoneColumns))
	require.Equal(t, 10, tombstoneColumns, "the minimal tombstone contains only the accepted ten fields")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='space_deletion_tombstones' AND (column_name ILIKE '%legal%' OR column_name ILIKE '%hold%')`).Scan(&forbidden))
	require.Zero(t, forbidden, "P3 tombstones expose no legal-hold authority")
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid='space_deletion_tombstones'::regclass AND contype='f')`).Scan(&hasForeignKey))
	require.False(t, hasForeignKey, "the tombstone must survive deletion of the Space row without a foreign key")

	applyLifecycleMigration(t, ctx, pool, "down")
	requireLifecycleSchema(t, pool, false)
	applyLifecycleMigration(t, ctx, pool, "up")
	requireLifecycleSchema(t, pool, true)
}

func TestLifecycleMigration_ConstrainsRawHashesParticipantsAndOutboxState(t *testing.T) {
	st := lifecycleStoreFixture(t)
	ctx := context.Background()
	store := requireDurableLifecycleStore(t, st)
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, store.PersistLifecycle(ctx, aggregate))

	for _, mutation := range []struct {
		name, sql string
	}{
		{"request hash length", `UPDATE space_lifecycle_participants SET request_sha256=decode('01','hex') WHERE space_id=$1`},
		{"receipt hash length", `UPDATE space_lifecycle_participants SET receipt_sha256=decode('01','hex') WHERE space_id=$1`},
		{"participant zero", `UPDATE space_lifecycle_participants SET participant_id=0 WHERE space_id=$1`},
		{"participant unknown", `UPDATE space_lifecycle_participants SET participant_id=11 WHERE space_id=$1`},
		{"outbox state unknown", `UPDATE space_lifecycle_outbox SET state='ACKED' WHERE space_id=$1`},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			_, err := st.Pool.Exec(ctx, mutation.sql, spaceID)
			require.Error(t, err)
		})
	}
}

func lifecycleEvidenceFingerprint(t *testing.T, pool *pgxpool.Pool) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	tables := []string{"space_lifecycle_operations", "space_lifecycle_aggregates", "space_lifecycle_participants", "space_lifecycle_outbox", "space_deletion_tombstones"}
	fingerprint := make([]string, 0, len(tables))
	for _, table := range tables {
		var digest string
		require.NoError(t, pool.QueryRow(ctx, `SELECT md5(COALESCE(string_agg(to_jsonb(row_value)::text,'|' ORDER BY to_jsonb(row_value)::text),'')) FROM `+table+` row_value`).Scan(&digest))
		fingerprint = append(fingerprint, digest)
	}
	return fingerprint
}

func assertLifecycleDownRefusesAndPreserves(t *testing.T, st *SpaceStore) {
	t.Helper()
	before := lifecycleEvidenceFingerprint(t, st.Pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := os.ReadFile(lifecycleMigrationPath(t, "down"))
	require.NoError(t, err)
	conn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	_, err = conn.Exec(ctx, string(raw))
	require.Error(t, err)
	var refusal *pgconn.PgError
	require.ErrorAs(t, err, &refusal)
	require.Equal(t, "P0001", refusal.Code)
	rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), time.Second)
	_, _ = conn.Exec(rollbackCtx, "ROLLBACK")
	rollbackCancel()
	requireLifecycleSchema(t, st.Pool, true)
	require.Equal(t, before, lifecycleEvidenceFingerprint(t, st.Pool), "DOWN refusal must preserve every byte of durable evidence")
}

func TestLifecycleMigration_DownRefusesEveryEvidenceClassAndPreservesData(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*testing.T, *SpaceStore)
	}{
		{"schedule operation", func(t *testing.T, st *SpaceStore) { reserveLifecycleOperationFixture(t, st) }},
		{"restore decision", func(t *testing.T, st *SpaceStore) {
			store, _, spaceID, _, _ := completeStoredSchedule(t, st)
			_, err := store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
			require.NoError(t, err)
		}},
		{"purge decision", func(t *testing.T, st *SpaceStore) {
			store, _, spaceID, _, _ := completeStoredSchedule(t, st)
			_, err := st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_aggregates SET purge_after=clock_timestamp() WHERE space_id=$1`, spaceID)
			require.NoError(t, err)
			_, err = store.DecideLifecycleRecovery(context.Background(), spaceID, 2)
			require.NoError(t, err)
		}},
		{"tombstone", func(t *testing.T, st *SpaceStore) {
			store, aggregate, _, _ := storedPurgingLifecycleFixture(t, st)
			_, err := aggregate.CompletePurge(time.Now().UTC())
			require.NoError(t, err)
			require.NoError(t, store.CompleteLifecyclePurge(context.Background(), aggregate, bytes.Repeat([]byte{0xa1}, 32), bytes.Repeat([]byte{0xb2}, 32), "space-tombstone-k7"))
		}},
		{"auth receipt", func(t *testing.T, st *SpaceStore) {
			store, _, _, actorProfileID, _, request, receiptBytes, receiptHash := reserveLifecycleOperationFixture(t, st)
			_, err := store.RecordLifecycleDeletionProofReceipt(context.Background(), actorProfileID, uuid.MustParse(request.GetOperationId()), receiptBytes, receiptHash)
			require.NoError(t, err)
		}},
		{"participant receipt", func(t *testing.T, st *SpaceStore) {
			store := requireDurableLifecycleStore(t, st)
			aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
			require.NoError(t, aggregate.RecordFenceReceipt(lifecycleFenceReceiptFixture(t, spaceID, operationID, commonv1.ParticipantId_PARTICIPANT_ID_CHAT, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())))
			require.NoError(t, store.PersistLifecycle(context.Background(), aggregate))
		}},
		{"ready outbox", func(t *testing.T, st *SpaceStore) { completeStoredSchedule(t, st) }},
		{"delivered outbox", func(t *testing.T, st *SpaceStore) {
			_, _, spaceID, _, _ := completeStoredSchedule(t, st)
			_, err := st.Pool.Exec(context.Background(), `UPDATE space_lifecycle_outbox SET state='DELIVERED',delivered_at=clock_timestamp() WHERE space_id=$1 AND event_type='space.deletion_scheduled'`, spaceID)
			require.NoError(t, err)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			st := lifecycleStoreFixture(t)
			test.setup(t, st)
			assertLifecycleDownRefusesAndPreserves(t, st)
		})
	}
}

func waitLifecycleDownForConcurrentInsert(t *testing.T, ctx context.Context, pool *pgxpool.Pool, pid int32) {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=$1 AND locktype='relation' AND mode='AccessExclusiveLock' AND NOT granted AND relation IN (
			'space_lifecycle_operations'::regclass,'space_lifecycle_aggregates'::regclass,'space_lifecycle_participants'::regclass,'space_lifecycle_outbox'::regclass,'space_deletion_tombstones'::regclass))`, pid).Scan(&waiting))
		if waiting {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("000013 DOWN never waited for the concurrent lifecycle insert: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestLifecycleMigration_DownWaitsForConcurrentCommitThenRefusesWithoutLoss(t *testing.T) {
	st := lifecycleStoreFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	aggregate, spaceID, operationID := lifecycleAggregateFixture(t, st)
	receipt := lifecycleFenceReceiptFixture(t, spaceID, operationID, commonv1.ParticipantId_PARTICIPANT_ID_CHAT, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture())
	require.NoError(t, aggregate.RecordFenceReceipt(receipt))
	insertTx, err := st.Pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = insertTx.Rollback(context.Background()) }()
	txStore := requireDurableLifecycleStore(t, &SpaceStore{Pool: st.Pool, tx: insertTx})
	require.NoError(t, txStore.PersistLifecycle(ctx, aggregate))

	raw, err := os.ReadFile(lifecycleMigrationPath(t, "down"))
	require.NoError(t, err)
	downConn, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	var pid int32
	require.NoError(t, downConn.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&pid))
	result := make(chan error, 1)
	done := make(chan struct{})
	defer func() {
		cancel()
		select {
		case <-done:
			downConn.Release()
		case <-time.After(3 * time.Second):
			t.Error("000013 DOWN worker did not terminate")
		}
	}()
	go func() {
		defer close(done)
		_, downErr := downConn.Exec(ctx, string(raw))
		if downErr != nil {
			rollbackCtx, rollbackCancel := context.WithTimeout(context.Background(), time.Second)
			_, _ = downConn.Exec(rollbackCtx, "ROLLBACK")
			rollbackCancel()
		}
		result <- downErr
	}()
	waitLifecycleDownForConcurrentInsert(t, ctx, st.Pool, pid)
	require.NoError(t, insertTx.Commit(ctx))
	downErr := <-result
	require.Error(t, downErr)
	var refusal *pgconn.PgError
	require.ErrorAs(t, downErr, &refusal)
	require.Equal(t, "P0001", refusal.Code)
	requireLifecycleSchema(t, st.Pool, true)
	loaded, err := requireDurableLifecycleStore(t, st).LoadLifecycle(ctx, spaceID)
	require.NoError(t, err)
	require.Equal(t, aggregate.Phase(), loaded.Phase())
	require.Equal(t, aggregate.Generation(), loaded.Generation())
	var rows int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_participants WHERE space_id=$1 AND deletion_operation_id=$2`, spaceID, operationID).Scan(&rows))
	require.Equal(t, 1, rows)
}
