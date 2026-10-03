package store

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/space/internal/spacecore"
)

type lifecycleFinalizer interface {
	FinalizeLifecyclePurge(context.Context, uuid.UUID, AccountHasher) (*spacecore.LifecycleAggregate, error)
}

type testLifecycleHasher struct{}

func (testLifecycleHasher) HashAccount(_ context.Context, id uuid.UUID) ([]byte, string, error) {
	return append(bytes.Repeat([]byte{0x73}, 16), id[:]...), "fixture-v1", nil
}

func TestLifecycleFinalizeDeletesLocalDataAtomicallyAndKeepsUnrelatedSpace(t *testing.T) {
	st := lifecycleStoreFixture(t)
	ctx := context.Background()
	for _, name := range []string{"000014_ownership_outbox_delivery.up.sql", "000015_audit_ledger.up.sql", "000022_lifecycle_runtime.up.sql", "000023_lifecycle_evidence_retention.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	durable, _, _, actor, spaceID, request, proof, hash := reserveLifecycleOperationFixture(t, st)
	operationID := uuid.MustParse(request.OperationId)
	_, err := durable.RecordLifecycleDeletionProofReceipt(ctx, actor, operationID, proof, hash)
	require.NoError(t, err)
	aggregate, err := st.BeginLifecycleFreeze(ctx, spaceID, lifecycleManifestFixture())
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	_, _, err = st.CompleteLifecycleSchedule(ctx, spaceID)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	aggregate, err = st.DecideLifecycleRecovery(ctx, spaceID, 2)
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, spaceID, operationID, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	snapshot := aggregate.Snapshot()
	require.NoError(t, aggregate.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, spaceID, operationID, 2, snapshot.PurgeDecidedAt)))
	for _, id := range lifecycleTestParticipants[1:] {
		require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, spaceID, operationID, id, 2, snapshot.PurgeDecidedAt)))
	}
	require.NoError(t, st.PersistLifecycle(ctx, aggregate))
	other, err := st.CreateSpace(ctx, uuid.New(), "unrelated", "", "private")
	require.NoError(t, err)
	otherID := other.ID
	for _, id := range []uuid.UUID{spaceID, otherID} {
		_, err = st.Pool.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details) VALUES($1,$2,$3,'invite_revoked','invite',$4,'{}')`, uuid.New(), id, uuid.New(), uuid.New())
		require.NoError(t, err)
	}
	finalizer, ok := any(st).(lifecycleFinalizer)
	require.True(t, ok, "production local purge must revalidate saved receipts and sample DB time under lock")
	_, err = finalizer.FinalizeLifecyclePurge(ctx, spaceID, nil)
	require.Error(t, err)
	_, err = st.Pool.Exec(ctx, `DELETE FROM spaces WHERE id=$1`, spaceID)
	require.Error(t, err, "direct delete before local completion must remain denied")
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION fail_local_purge() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture local purge failure'; END $$; CREATE TRIGGER fail_local_purge BEFORE DELETE ON spaces FOR EACH ROW EXECUTE FUNCTION fail_local_purge()`)
	require.NoError(t, err)
	_, err = finalizer.FinalizeLifecyclePurge(ctx, spaceID, testLifecycleHasher{})
	require.Error(t, err)
	var exists bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM spaces WHERE id=$1) AND NOT EXISTS(SELECT 1 FROM space_deletion_tombstones WHERE space_id=$1)`, spaceID).Scan(&exists))
	require.True(t, exists, "local error must roll back tombstone, event and deletion")
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER fail_local_purge ON spaces; DROP FUNCTION fail_local_purge()`)
	require.NoError(t, err)
	var before time.Time
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&before))
	completed, err := finalizer.FinalizeLifecyclePurge(ctx, spaceID, testLifecycleHasher{})
	require.NoError(t, err)
	require.False(t, completed.Snapshot().DeletedEvent.OccurredAt.Before(before))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM spaces WHERE id=$1) AND NOT EXISTS(SELECT 1 FROM audit_log WHERE space_id=$1) AND EXISTS(SELECT 1 FROM spaces WHERE id=$2) AND EXISTS(SELECT 1 FROM audit_log WHERE space_id=$2) AND EXISTS(SELECT 1 FROM space_lifecycle_outbox WHERE space_id=$1 AND event_type='space.deleted' AND state='READY')`, spaceID, otherID).Scan(&exists))
	require.True(t, exists)
	replay, err := finalizer.FinalizeLifecyclePurge(ctx, spaceID, testLifecycleHasher{})
	require.NoError(t, err)
	require.Equal(t, completed.Snapshot().DeletedEvent, replay.Snapshot().DeletedEvent)
	require.NoError(t, st.CleanupLifecycleEvidence(ctx))
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_completions WHERE space_id=$1`, spaceID).Scan(&count))
	require.Zero(t, count, "private evidence must survive the first thirty days")
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_aggregates SET completed_at=clock_timestamp()-interval '30 days 1 second' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	require.NoError(t, st.CleanupLifecycleEvidence(ctx))
	require.NoError(t, st.CleanupLifecycleEvidence(ctx))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_completions WHERE space_id=$1`, spaceID).Scan(&count))
	require.Equal(t, 10, count)
	for _, table := range []string{"space_lifecycle_operations", "space_lifecycle_participants"} {
		require.NoError(t, st.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE space_id=$1", spaceID).Scan(&count))
		require.Zero(t, count, table)
	}
	compactReplay, err := finalizer.FinalizeLifecyclePurge(ctx, spaceID, testLifecycleHasher{})
	require.NoError(t, err)
	require.Equal(t, completed.Snapshot().DeletedEvent, compactReplay.Snapshot().DeletedEvent)
	_, err = st.Pool.Exec(ctx, `UPDATE space_deletion_tombstones SET scheduled_at=scheduled_at-interval '366 days',purge_after=purge_after-interval '366 days',purge_decided_at=purge_decided_at-interval '366 days',purged_at=purged_at-interval '366 days',retain_until=retain_until-interval '366 days' WHERE space_id=$1`, spaceID)
	require.NoError(t, err)
	require.NoError(t, st.CleanupLifecycleEvidence(ctx))
	for _, table := range []string{"space_lifecycle_completions", "space_lifecycle_aggregates", "space_deletion_tombstones"} {
		require.NoError(t, st.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE space_id=$1", spaceID).Scan(&count))
		require.Zero(t, count, table)
	}
	record := *completed.Snapshot().DeletedEvent
	decision, err := st.LifecycleEventPurgeDecision(ctx, record)
	require.NoError(t, err)
	require.Equal(t, snapshot.PurgeDecidedAt, decision, "READY event remains dispatchable after aggregate and HMAC expiry")
	require.NoError(t, st.MarkLifecycleEventDelivered(ctx, record))
	require.NoError(t, st.CleanupLifecycleEvidence(ctx))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_outbox WHERE event_id=$1`, record.EventID).Scan(&count))
	require.Equal(t, 1, count)
	_, err = st.Pool.Exec(ctx, `UPDATE space_lifecycle_outbox SET delivered_at=clock_timestamp()-interval '30 days 1 second' WHERE event_id=$1`, record.EventID)
	require.NoError(t, err)
	require.NoError(t, st.CleanupLifecycleEvidence(ctx))
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT count(*) FROM space_lifecycle_outbox WHERE event_id=$1`, record.EventID).Scan(&count))
	require.Zero(t, count, "delivered wire evidence expires at the first ACK plus thirty days")
}
