package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/spacecore"
)

func TestNativeTerminal47ConcurrentExpiryAndSavepointIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL18 actual terminal clock/cohort47")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), nativeTerminalClockImage(t))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	spaces := make([]uuid.UUID, 2)
	for i := range spaces {
		f := newLifecycleOutcomeFixtureWithStore(t, admin, len(lifecycleTestParticipants))
		spaces[i] = f.space
		_, _, err := admin.CompleteLifecycleSchedule(ctx, f.space)
		require.NoError(t, err)
		_, err = admin.Pool.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, f.space)
		require.NoError(t, err)
		a, err := admin.DecideLifecycleRecovery(ctx, f.space, 2)
		require.NoError(t, err)
		op := uuid.MustParse(f.request.OperationId)
		recordLifecycleFenceBarrier(t, a, f.space, op, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
		require.NoError(t, a.BeginPurging())
		require.NoError(t, a.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, f.space, op, 2, a.Snapshot().PurgeDecidedAt)))
		for _, participant := range lifecycleTestParticipants[1:] {
			require.NoError(t, a.RecordPurgeReceipt(lifecyclePurgeReceipt(t, f.space, op, participant, 2, a.Snapshot().PurgeDecidedAt)))
		}
		require.NoError(t, admin.PersistLifecycle(ctx, a))
	}
	db.Run(true, "goto", "46")
	_, err := admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	for _, space := range spaces {
		require.NoError(t, admin.InitializeNativeMembership(ctx, space))
	}
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, admin.Pool)}
	first := time.Now().UTC().Add(-time.Second).Truncate(time.Second)
	setNativeTerminalDBTime(t, admin, first)
	events := make([]spacecore.LifecycleOutboxRecord, 2)
	for i, space := range spaces {
		a, err := serving.FinalizeLifecyclePurge(ctx, space, testLifecycleHasher{})
		require.NoError(t, err)
		events[i] = *a.Snapshot().DeletedEvent
	}
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	t.Run("savepoint rollback and foreign Space cannot borrow scope", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving.Pool, spaces[0], func(tx pgx.Tx) error {
			nested, err := tx.Begin(ctx)
			require.NoError(t, err)
			_, err = nested.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`)
			require.NoError(t, err)
			raw := marshalLifecycleOutbox(events[0])
			hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", raw)
			_, err = nested.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, spaces[0], events[0].EventID, raw, hash[:])
			require.NoError(t, err)
			require.NoError(t, nested.Rollback(ctx))
			var ready bool
			require.NoError(t, tx.QueryRow(ctx, `SELECT state='READY' AND delivered_at IS NULL FROM public.space_lifecycle_outbox WHERE event_id=$1`, events[0].EventID).Scan(&ready))
			require.True(t, ready)
			foreign, err := tx.Begin(ctx)
			require.NoError(t, err)
			raw = marshalLifecycleOutbox(events[1])
			hash = lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", raw)
			_, err = foreign.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, spaces[1], events[1].EventID, raw, hash[:])
			require.Error(t, err, "the actual A fence cannot authorize B")
			require.NoError(t, foreign.Rollback(ctx))
			_, err = tx.Exec(ctx, `UPDATE public.space_lifecycle_outbox SET state='DELIVERED',delivered_at=clock_timestamp() WHERE event_id=$1`, events[0].EventID)
			require.Error(t, err, "rollback cannot leave a usable helper scope")
			return err
		})
		require.Error(t, err)
		var ready int
		require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_lifecycle_outbox WHERE event_id=ANY($1) AND state='READY' AND delivered_at IS NULL`, []string{events[0].EventID, events[1].EventID}).Scan(&ready))
		require.Equal(t, 2, ready)
	})
	t.Run("actual queued ACK compaction expiry preserve first clock", func(t *testing.T) {
		setNativeTerminalDBTime(t, admin, first.Add(365*24*time.Hour))
		blocker, err := serving.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, err)
		defer blocker.Rollback(ctx)
		require.NoError(t, lockLifecycleSpace(ctx, blocker, spaces[0]))
		var blockerPID int32
		require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&blockerPID))
		results := make(chan error, 3)
		go func() { results <- serving.MarkLifecycleEventDelivered(ctx, events[0]) }()
		for _, helper := range []string{"space_native_terminal_compact", "space_native_terminal_expire"} {
			helper := helper
			go func() {
				results <- nativeReadinessTx(ctx, serving.Pool, spaces[0], func(tx pgx.Tx) error { _, err := tx.Exec(ctx, `SELECT public.`+helper+`($1)`, spaces[0]); return err })
			}()
		}
		var queued int
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid))`, blockerPID).Scan(&queued))
			if queued == 3 {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.Equal(t, 3, queued, "all three real operations must contend on the actual Space fence")
		require.NoError(t, blocker.Commit(ctx))
		for range 3 {
			require.NoError(t, <-results)
		}
		require.NoError(t, serving.CleanupLifecycleEvidence(ctx), "multi-Space bounded cleanup must also process B under B's fence")
		var complete bool
		require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM native_membership_guard.spaces WHERE space_id=ANY($1) AND terminal_maintenance_phase='EXPIRED')=2
 AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates WHERE space_id=ANY($1))
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.terminal_maintenance_scope)
 AND EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE event_id=$2 AND state='DELIVERED' AND delivered_at=$4)
 AND EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE event_id=$3 AND state='READY' AND delivered_at IS NULL)`, spaces, events[0].EventID, events[1].EventID, first.Add(365*24*time.Hour)).Scan(&complete))
		require.True(t, complete)
		require.NoError(t, serving.MarkLifecycleEventDelivered(ctx, events[0]))
	})
}
