package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/space/internal/spacecore"
)

// This starts with the real official34 lifecycle and accepted typed current11
// evidence, then uses the real native owner for P3. It does not seed a terminal
// registry row, suppress a guard or manufacture a compact disposition.
func nativeTerminalMaintenanceFixture(t *testing.T) (*SpaceStore, *SpaceStore, spacecore.LifecycleOutboxRecord) {
	return nativeTerminalMaintenanceFixtureWithImage(t, "postgres:18-alpine")
}

func nativeTerminalMaintenanceFixtureWithImage(t *testing.T, image string, purgeUnder47 ...bool) (*SpaceStore, *SpaceStore, spacecore.LifecycleOutboxRecord) {
	return nativeTerminalMaintenanceFixtureHook(t, image, nil, nil, purgeUnder47...)
}

func nativeTerminalMaintenanceFixtureHook(t *testing.T, image string, before47 func(*SpaceStore, *SpaceStore, spacecore.LifecycleOutboxRecord), beforePurge func(*SpaceStore), purgeUnder47 ...bool) (*SpaceStore, *SpaceStore, spacecore.LifecycleOutboxRecord) {
	t.Helper()
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), image)
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	f := newLifecycleOutcomeFixtureWithStore(t, admin, len(lifecycleTestParticipants))
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
	// Explicit46 is the causal baseline. Forward47 tests will upgrade only after
	// recording this real current11 terminal state and its native P3 receipt.
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, f.space))
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, admin.Pool)}
	upgrade := func() {
		db.Run(true, "up", "1")
		_, err = admin.TransitionNativeMembershipCatalog47(ctx)
		require.NoError(t, err, "trusted forward46-to47 transition must preserve the native P3 protocol")
	}
	if len(purgeUnder47) > 0 && purgeUnder47[0] {
		upgrade()
	}
	if beforePurge != nil {
		beforePurge(admin)
	}
	completed, err := serving.FinalizeLifecyclePurge(ctx, f.space, testLifecycleHasher{})
	require.NoError(t, err)
	if before47 != nil {
		before47(admin, serving, *completed.Snapshot().DeletedEvent)
	}
	if len(purgeUnder47) == 0 || !purgeUnder47[0] {
		upgrade()
	}
	return admin, serving, *completed.Snapshot().DeletedEvent
}

func TestNativeTerminal47P3AndAckDeadlinePreserveNegativeDisposition(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL18/current47 P3 and real held-row wait")
	}
	admin, serving, event := nativeTerminalMaintenanceFixtureWithImage(t, "postgres:18-alpine", true)
	ctx := context.Background()
	space := uuid.MustParse(event.SpaceID)
	t.Run("ambient RR rejected", func(t *testing.T) {
		tx, err := serving.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		require.NoError(t, err)
		defer tx.Rollback(ctx)
		require.NoError(t, lockLifecycleSpace(ctx, tx, space))
		raw := marshalLifecycleOutbox(event)
		hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", raw)
		_, err = tx.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, space, event.EventID, raw, hash[:])
		require.Error(t, err)
	})
	t.Run("missing fence and consumed scope", func(t *testing.T) {
		raw := marshalLifecycleOutbox(event)
		hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", raw)
		_, err := serving.Pool.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, space, event.EventID, raw, hash[:])
		require.Error(t, err, "an unfenced backend cannot borrow another session's authority")
		err = nativeReadinessTx(ctx, serving.Pool, space, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SET CONSTRAINTS ALL IMMEDIATE`); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, space, event.EventID, raw, hash[:]); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE public.space_lifecycle_outbox SET state='READY',delivered_at=NULL WHERE event_id=$1`, event.EventID)
			return err
		})
		require.Error(t, err, "completed fixed helper must leave no reusable scope for later caller SQL")
		_, err = serving.Pool.Exec(ctx, `UPDATE native_membership_guard.spaces SET terminal_maintenance_phase='EXPIRED' WHERE space_id=$1`, space)
		require.Error(t, err, "serving credentials cannot manufacture minimization provenance")
	})
	t.Run("temporary shadow rejected", func(t *testing.T) {
		err := nativeReadinessTx(ctx, serving.Pool, space, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `CREATE TEMP TABLE space_lifecycle_outbox(event_id uuid)`); err != nil {
				return err
			}
			raw := marshalLifecycleOutbox(event)
			hash := lifecycleHash("voice.space.store.v1.LifecycleOutboxRecord", raw)
			_, err := tx.Exec(ctx, `SELECT public.space_native_terminal_event_delivered($1,$2,$3,$4)`, space, event.EventID, raw, hash[:])
			return err
		})
		require.Error(t, err)
	})
	t.Run("held ACK rollback and fresh retry", func(t *testing.T) {
		blocker, err := admin.Pool.Begin(ctx)
		require.NoError(t, err)
		defer blocker.Rollback(ctx)
		var pid int32
		require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid() FROM public.space_lifecycle_outbox WHERE event_id=$1 FOR UPDATE`, event.EventID).Scan(&pid))
		done := make(chan error, 1)
		go func() { done <- serving.MarkLifecycleEventDelivered(ctx, event) }()
		blocked := false
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked))
			if blocked {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.True(t, blocked, "must observe actual PostgreSQL row wait before measuring deadline")
		time.Sleep(5050 * time.Millisecond)
		require.NoError(t, blocker.Commit(ctx))
		require.Error(t, <-done)
		var intact bool
		require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE event_id=$1 AND state='READY' AND delivered_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.terminal_maintenance_scope)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$2)`, event.EventID, space).Scan(&intact))
		require.True(t, intact)
		require.NoError(t, serving.MarkLifecycleEventDelivered(ctx, event))
	})
}

// The image is the same PostgreSQL18 binary plus test-only libfaketime0.9.13.
// Changing its process-local clock file never updates product evidence, disables
// a trigger, changes a helper body, or changes the host clock. Every elapsed
// boundary below is exercised by the real clock_timestamp() predicates.
func setNativeTerminalDBTime(t *testing.T, admin *SpaceStore, at time.Time) {
	t.Helper()
	at = at.UTC()
	require.Zero(t, at.Nanosecond(), "boundary fixture chooses its first P3 clock on an exactly representable whole second")
	value := at.Format("2006-01-02 15:04:05")
	// Atomic rename never exposes a truncated clock file to another PG process.
	_, err := admin.Pool.Exec(context.Background(), "COPY (SELECT '"+value+"') TO PROGRAM 'cat > /tmp/native-terminal-clock.next && mv /tmp/native-terminal-clock.next /tmp/native-terminal-clock'")
	require.NoError(t, err)
	var actual time.Time
	require.NoError(t, admin.Pool.QueryRow(context.Background(), `SELECT clock_timestamp()`).Scan(&actual))
	require.Equal(t, at, actual.UTC(), "test clock must actually hold the exact DB microsecond boundary")
}

func nativeTerminalClockImage(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	integrationtest.ConfigureDockerTesting()
	tag := uuid.NewString()
	image := "voice-native-terminal-clock:" + tag
	process, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{Context: filepath.Join(repoRoot(t), "src/backend/space/internal/store/testdata/terminal_clock"), Dockerfile: "Dockerfile", Repo: "voice-native-terminal-clock", Tag: tag},
			Cmd:            []string{"true"}, WaitingFor: wait.ForExit(),
		}, Started: true,
	})
	if process != nil {
		t.Cleanup(func() {
			cleanup, stop := context.WithTimeout(context.Background(), 30*time.Second)
			defer stop()
			require.NoError(t, process.Terminate(cleanup))
		})
	}
	require.NoError(t, err)
	return image
}

func TestNativeTerminalMaintenanceCompactionExpiryAndLateReadyDelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private PostgreSQL18 test clock image")
	}
	admin, serving, event := nativeTerminalMaintenanceFixtureHook(t, nativeTerminalClockImage(t), nil, func(admin *SpaceStore) {
		// Choose the test process clock before the actual P3 owner samples its
		// first completion, rather than rewriting any persisted first clock.
		// Whole seconds avoid libfaketime's fractional parsing quantization.
		setNativeTerminalDBTime(t, admin, time.Now().UTC().Add(-time.Second).Truncate(time.Second))
	})
	ctx := context.Background()
	var fullDeadline, tombstoneDeadline time.Time
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT a.completed_at+interval '30 days',t.retain_until AT TIME ZONE 'UTC'
 FROM public.space_lifecycle_aggregates a JOIN public.space_deletion_tombstones t USING(space_id) WHERE a.space_id=$1`, event.SpaceID).Scan(&fullDeadline, &tombstoneDeadline))
	// Premature cleanup must neither copy evidence nor alter first timestamps.
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	var count int
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_lifecycle_completions WHERE space_id=$1`, event.SpaceID).Scan(&count))
	require.Zero(t, count)
	var expected string
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_array(space_id,deletion_operation_id,generation,participant_id,request_kind,
 encode(request_sha256,'hex'),encode(receipt_sha256,'hex'),encode(manifest_sha256,'hex'),completed_at) ORDER BY participant_id)::text
 FROM public.space_lifecycle_participants WHERE space_id=$1 AND ((participant_id=1 AND request_kind='ROLE_RETIREMENT') OR (participant_id BETWEEN 2 AND 11 AND request_kind='PURGE'))`, event.SpaceID).Scan(&expected))
	setNativeTerminalDBTime(t, admin, fullDeadline.Add(-time.Second))
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_lifecycle_completions WHERE space_id=$1`, event.SpaceID).Scan(&count))
	require.Zero(t, count, "before the original30-day completion deadline is premature")
	setNativeTerminalDBTime(t, admin, fullDeadline)
	t.Run("cancel after actual copy and source delete rolls everything back", func(t *testing.T) {
		blocker, err := admin.Pool.Begin(ctx)
		require.NoError(t, err)
		defer blocker.Rollback(ctx)
		var pid int32
		require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid() FROM public.space_lifecycle_operations WHERE space_id=$1 AND method='DELETE' LIMIT 1 FOR UPDATE`, event.SpaceID).Scan(&pid))
		operationCtx, cancel := context.WithCancel(ctx)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- serving.CleanupLifecycleEvidence(operationCtx) }()
		blocked := false
		for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
			require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, pid).Scan(&blocked))
			if blocked {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.True(t, blocked, "fixed compact SQL reaches the actual operation DELETE after copying tuples and deleting participant bodies")
		cancel()
		require.Error(t, <-done)
		require.NoError(t, blocker.Commit(ctx))
		var intact bool
		require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT public.space_terminal_purge_ready($1)
 AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_completions WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.terminal_maintenance_scope)
 AND NOT evidence_compacted FROM public.space_lifecycle_aggregates WHERE space_id=$1 FOR UPDATE`, event.SpaceID).Scan(&intact))
		require.True(t, intact, "no partial copy/delete/flag/certification survives cancellation")
	})
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	var actual string
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT jsonb_agg(jsonb_build_array(space_id,deletion_operation_id,generation,participant_id,request_kind,
 encode(request_sha256,'hex'),encode(receipt_sha256,'hex'),encode(manifest_sha256,'hex'),completed_at) ORDER BY participant_id)::text
 FROM public.space_lifecycle_completions WHERE space_id=$1`, event.SpaceID).Scan(&actual))
	require.Equal(t, expected, actual, "all eleven original tuple/hash/first-clock values must be copied exactly")
	var certified bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT NOT public.space_terminal_purge_ready($1)
 AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=$1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND terminal_maintenance_phase='COMPACT')`, event.SpaceID).Scan(&certified))
	require.True(t, certified, "maintenance disposition must not broaden the old full-row positive predicate")
	setNativeTerminalDBTime(t, admin, tombstoneDeadline.Add(-time.Second))
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_deletion_tombstones WHERE space_id=$1)`, event.SpaceID).Scan(&certified))
	require.True(t, certified)
	setNativeTerminalDBTime(t, admin, tombstoneDeadline)
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM public.space_deletion_tombstones WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_completions WHERE space_id=$1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND terminal_maintenance_phase='EXPIRED')
 AND EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE event_id=$2 AND state='READY')`, event.SpaceID, event.EventID).Scan(&certified))
	require.True(t, certified)
	visited := map[string]bool{}
	require.NoError(t, serving.ReadReadyLifecycleOutbox(ctx, 100, func(saved spacecore.LifecycleOutboxRecord) error {
		if saved.SpaceID != event.SpaceID {
			return nil
		}
		if saved.EventType == "space.deleted" {
			_, err := serving.LifecycleEventPurgeDecision(ctx, saved)
			if err != nil {
				return err
			}
		}
		// The production worker performs its remote PublishLifecycle here,
		// outside SQL fences. This callback is a local delivery-success control,
		// not a claim of broker acceptance or remote consumer effects.
		visited[saved.EventType] = true
		return serving.MarkLifecycleEventDelivered(ctx, saved)
	}), "actual Store dispatcher sequence must retain READY delivery after HMAC/aggregate expiry")
	require.True(t, visited["space.deleted"])
	require.True(t, visited["space.deletion_scheduled"], "original historical generation must not be rewritten to the later purge generation")
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_lifecycle_outbox WHERE event_id=$1`, event.EventID).Scan(&count))
	require.Equal(t, 1, count, "the ACK clock is independent from the old purge deadline")
	var ackDeadline time.Time
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT delivered_at+interval '30 days' FROM public.space_lifecycle_outbox WHERE event_id=$1`, event.EventID).Scan(&ackDeadline))
	setNativeTerminalDBTime(t, admin, ackDeadline.Add(-time.Second))
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_lifecycle_outbox WHERE event_id=$1`, event.EventID).Scan(&count))
	require.Equal(t, 1, count)
	setNativeTerminalDBTime(t, admin, ackDeadline)
	require.NoError(t, serving.CleanupLifecycleEvidence(ctx))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_lifecycle_outbox WHERE event_id=$1`, event.EventID).Scan(&count))
	require.Zero(t, count)
	require.Error(t, serving.MarkLifecycleEventDelivered(ctx, event), "expiry must never reinsert an acknowledged event")
}

func TestNativeTerminalMaintenanceCannotEraseUncertifiedFullEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private Linux PostgreSQL18 and actual current11 P3")
	}
	_, serving, event := nativeTerminalMaintenanceFixture(t)
	space := uuid.MustParse(event.SpaceID)
	err := nativeReadinessTx(context.Background(), serving.Pool, space, func(tx pgx.Tx) error {
		_, err := tx.Exec(context.Background(), `DELETE FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=11 AND request_kind='PURGE'`, space)
		return err
	})
	require.Error(t, err, "raw terminal DELETE must not erase current11 before certified compaction")
	var count int
	require.NoError(t, serving.Pool.QueryRow(context.Background(), `SELECT count(*) FROM public.space_lifecycle_participants WHERE space_id=$1 AND request_kind IN ('PURGE','ROLE_RETIREMENT')`, space).Scan(&count))
	require.Equal(t, 11, count)
}

func TestNativeTerminal47DoesNotAdoptMissingHistoricalCurrent11(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real retained46-to47 missing-evidence target")
	}
	ctx := context.Background()
	admin, serving, event := nativeTerminalMaintenanceFixtureHook(t, "postgres:18-alpine", func(_ *SpaceStore, old *SpaceStore, event spacecore.LifecycleOutboxRecord) {
		// Reproduce the actual old46 direct-delete gap before47 installation.
		// The lost evidence is never reinserted or relabeled as certified.
		require.NoError(t, nativeReadinessTx(ctx, old.Pool, uuid.MustParse(event.SpaceID), func(tx pgx.Tx) error {
			_, err := tx.Exec(ctx, `DELETE FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=11 AND request_kind='PURGE'`, event.SpaceID)
			return err
		}))
	}, nil)
	require.Error(t, serving.MarkLifecycleEventDelivered(ctx, event))
	require.Error(t, nativeReadinessTx(ctx, serving.Pool, uuid.MustParse(event.SpaceID), func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `SELECT public.space_native_terminal_compact($1)`, event.SpaceID)
		return err
	}))
	var unknown bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM public.space_lifecycle_participants WHERE space_id=$1 AND request_kind IN ('ROLE_RETIREMENT','PURGE'))=10
 AND EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND terminal_maintenance_phase IS NULL)
 AND EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE event_id=$2 AND state='READY' AND delivered_at IS NULL)
 AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_completions WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.terminal_maintenance_scope)`, event.SpaceID, event.EventID).Scan(&unknown))
	require.True(t, unknown, "new catalog admission cannot certify old missing provenance")
}

func TestNativeTerminalMaintenanceAcknowledgesExactSavedEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private Linux PostgreSQL18 and actual current11 P3")
	}
	admin, serving, event := nativeTerminalMaintenanceFixture(t)
	ctx := context.Background()
	wrong := event
	wrong.Generation++
	require.Error(t, serving.MarkLifecycleEventDelivered(ctx, wrong))
	wrong = event
	wrong.EventID = "not-a-uuid"
	require.Error(t, serving.MarkLifecycleEventDelivered(ctx, wrong))
	require.Error(t, admin.MarkLifecycleEventDelivered(ctx, event), "installer owner cannot borrow the serving helper")
	require.NoError(t, serving.MarkLifecycleEventDelivered(ctx, event), "exact saved READY ACK must work after native P3")
	var before, after string
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM public.space_lifecycle_outbox o WHERE event_id=$1`, event.EventID).Scan(&before))
	require.NoError(t, serving.MarkLifecycleEventDelivered(ctx, event))
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT to_jsonb(o)::text FROM public.space_lifecycle_outbox o WHERE event_id=$1`, event.EventID).Scan(&after))
	require.Equal(t, before, after, "exact replay preserves original bytes and first ACK time")
	var negative bool
	require.NoError(t, admin.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.terminal_maintenance_scope)
 AND EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND state='TERMINAL' AND terminal_maintenance_phase='FULL')`, event.SpaceID).Scan(&negative))
	require.True(t, negative)
}
