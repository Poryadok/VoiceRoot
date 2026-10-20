package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"path/filepath"
	"testing"
	"time"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeP3DrainCurrent48LiveBarrier(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	account, actor := uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, actor, "R23 durable lifecycle", "", "private")
	require.NoError(t, err)
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	call := signedNativeBotCaller(t, serving)
	profile := uuid.NewString()
	add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: profile, Generation: 1}}
	_, err = call(add)
	require.NoError(t, err)
	operation := uuid.New()
	request := lifecycleScheduleRequest(space.ID, operation)
	aggregate, err := serving.ReserveLifecycleSchedule(ctx, account, actor, 7, request)
	require.NoError(t, err, "actual guarded lifecycle owner must capture before first reservation SQL")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, aggregate.Phase())
	_, confirmation, digest := lifecycleScheduleRequestEvidence(t, request)
	proof, proofSHA := lifecycleAuthReceiptEvidence(t, account, actor, space.ID, operation, 7, confirmation, digest)
	_, err = serving.RecordLifecycleDeletionProofReceipt(ctx, actor, operation, proof, proofSHA)
	require.NoError(t, err, "proof persistence is another actual guarded lifecycle writer")
	var complete bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_lifecycle_operations WHERE operation_id=$1 AND auth_receipt_bytes=$2)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, operation, proof).Scan(&complete))
	require.True(t, complete)
	aggregate, err = serving.BeginLifecycleFreeze(ctx, space.ID, lifecycleManifestFixture())
	require.NoError(t, err, "actual guarded lifecycle freeze snapshot")
	for _, participant := range lifecycleTestParticipants {
		aggregate, err = serving.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, space.ID, operation, participant, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	_, _, err = serving.CompleteLifecycleSchedule(ctx, space.ID)
	require.NoError(t, err)
	// Test-only elapsed-seven-day fixture, inside the real serving DB frame.
	// No participant receipt, native source or history boundary is fabricated.
	require.NoError(t, nativeReadinessTx(ctx, serving.Pool, space.ID, func(tx pgx.Tx) error {
		scoped := &SpaceStore{Pool: serving.Pool, tx: tx}
		capture, err := scoped.captureLifecycleMaintenanceMutation(ctx, space.ID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space.ID); err != nil {
			return err
		}
		return scoped.finishLifecycleMaintenanceMutation(ctx, capture)
	}))
	aggregate, err = serving.DecideLifecycleRecovery(ctx, space.ID, 2)
	require.NoError(t, err)
	for _, participant := range lifecycleTestParticipants {
		aggregate, err = serving.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, space.ID, operation, participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, serving.PersistLifecycle(ctx, aggregate))
	_, err = serving.RecordLifecycleRoleRetirementReceipt(ctx, lifecycleRoleRetirementReceipt(t, space.ID, operation, 2, aggregate.Snapshot().PurgeDecidedAt))
	require.NoError(t, err)

	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: profile, InstallOperationId: add.InstallationIntent.OperationId, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent)}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	_, err = call(cancel)
	require.ErrorIs(t, err, ErrLifecycleFrozen, "ordinary cancellation retains its frozen-lifecycle guard; a distinct ownP3 negative-purpose method is required")
	var retained bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2) AND EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE install_operation_id=$3 AND state='PUBLISHED') AND NOT EXISTS(SELECT 1 FROM public.space_bot_installation_cancellations WHERE operation_id=$4) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, space.ID, profile, add.InstallationIntent.OperationId, cancel.InstallationIntent.OperationId).Scan(&retained))
	require.True(t, retained)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGING, aggregate.Phase())
	t.Log("actual48 native source preserved: ordinary LIVE-only cancellation denied after own irreversible P3 and typed Role-first; no Bot receipt/aggregate prerequisite introduced")
}

func TestNativeP3ChildTypedEffectBeforeBotReceipt(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), nativeTerminalClockImage(t))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	account, actor := uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, actor, "R23 durable lifecycle", "", "private")
	require.NoError(t, err)
	db.Run(true, "goto", "46")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog47(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog48(ctx)
	require.NoError(t, err)
	db.Run(true, "up", "1")
	_, err = admin.TransitionNativeMembershipCatalog49(ctx)
	require.NoError(t, err)
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	call := signedNativeBotCaller(t, serving)
	profile := uuid.NewString()
	add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: profile, Generation: 1}}
	_, err = call(add)
	require.NoError(t, err)
	operation := uuid.New()
	request := lifecycleScheduleRequest(space.ID, operation)
	aggregate, err := serving.ReserveLifecycleSchedule(ctx, account, actor, 7, request)
	require.NoError(t, err, "actual guarded lifecycle owner must capture before first reservation SQL")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_SCHEDULE_PENDING, aggregate.Phase())
	_, confirmation, digest := lifecycleScheduleRequestEvidence(t, request)
	proof, proofSHA := lifecycleAuthReceiptEvidence(t, account, actor, space.ID, operation, 7, confirmation, digest)
	_, err = serving.RecordLifecycleDeletionProofReceipt(ctx, actor, operation, proof, proofSHA)
	require.NoError(t, err, "proof persistence is another actual guarded lifecycle writer")
	var complete bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_lifecycle_operations WHERE operation_id=$1 AND auth_receipt_bytes=$2)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, operation, proof).Scan(&complete))
	require.True(t, complete)
	aggregate, err = serving.BeginLifecycleFreeze(ctx, space.ID, lifecycleManifestFixture())
	require.NoError(t, err, "actual guarded lifecycle freeze snapshot")
	for _, participant := range lifecycleTestParticipants {
		aggregate, err = serving.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, space.ID, operation, participant, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	_, _, err = serving.CompleteLifecycleSchedule(ctx, space.ID)
	require.NoError(t, err)
	// Test-only elapsed-seven-day fixture, inside the real serving DB frame.
	// No participant receipt, native source or history boundary is fabricated.
	require.NoError(t, nativeReadinessTx(ctx, serving.Pool, space.ID, func(tx pgx.Tx) error {
		scoped := &SpaceStore{Pool: serving.Pool, tx: tx}
		capture, err := scoped.captureLifecycleMaintenanceMutation(ctx, space.ID)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space.ID); err != nil {
			return err
		}
		return scoped.finishLifecycleMaintenanceMutation(ctx, capture)
	}))
	aggregate, err = serving.DecideLifecycleRecovery(ctx, space.ID, 2)
	require.NoError(t, err)
	for _, participant := range lifecycleTestParticipants {
		aggregate, err = serving.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, space.ID, operation, participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, serving.PersistLifecycle(ctx, aggregate))
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: profile, InstallOperationId: add.InstallationIntent.OperationId, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent)}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	child := &spacev1.ReconcileBotPurgeInstallationRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: space.ID.String(), DeletionOperationId: operation.String(), Generation: aggregate.Generation(), Manifest: proto.Clone(aggregate.Snapshot().Manifest).(*commonv1.ManifestBinding), BotId: add.InstallationIntent.BotId, InstallationId: add.InstallationIntent.InstallationId, NativeCancellation: cancel}
	childCall := signedNativeP3ChildCaller(t, serving)
	_, err = childCall(child)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "own irreversible decision without accepted Role-first does not authorize a child")
	var unchanged bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children)
 AND EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, space.ID, profile).Scan(&unchanged))
	require.True(t, unchanged)
	_, err = serving.RecordLifecycleRoleRetirementReceipt(ctx, lifecycleRoleRetirementReceipt(t, space.ID, operation, 2, aggregate.Snapshot().PurgeDecidedAt))
	require.NoError(t, err)

	for _, mode := range []string{"generation", "manifest", "parent", "source"} {
		t.Run("reject wrong "+mode, func(t *testing.T) {
			bad := proto.Clone(child).(*spacev1.ReconcileBotPurgeInstallationRequest)
			bad.OperationId = uuid.NewString()
			switch mode {
			case "generation":
				bad.Generation++
			case "manifest":
				bad.Manifest.ManifestSha256[0] ^= 1
			case "parent":
				bad.DeletionOperationId = uuid.NewString()
			case "source":
				bad.NativeCancellation.InstallationIntent.Generation++
			}
			_, err := childCall(bad)
			require.Error(t, err)
		})
	}
	// Hold an actual native source row after the child has acquired its own
	// operation/Space authority. This expires the DB five-second mutation
	// horizon, not the independent default thirty-second service JWT.
	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	var blockerPID int32
	require.NoError(t, blocker.QueryRow(ctx, `SELECT pg_backend_pid() FROM public.space_members WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, space.ID, profile).Scan(&blockerPID))
	done := make(chan error, 1)
	go func() { _, err := childCall(child); done <- err }()
	blocked := false
	for until := time.Now().Add(3 * time.Second); time.Now().Before(until); {
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))`, blockerPID).Scan(&blocked))
		if blocked {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.True(t, blocked, "observe actual source-row wait before elapsed deadline assertion")
	time.Sleep(5050 * time.Millisecond)
	require.NoError(t, blocker.Commit(ctx))
	require.Error(t, <-done)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children)
 AND EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2)
 AND EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE install_operation_id=$3 AND state='PUBLISHED')
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, space.ID, profile, add.InstallationIntent.OperationId).Scan(&unchanged))
	require.True(t, unchanged, "failed oldest call cannot leave native cancellation, child result or frame")
	legacy := proto.Clone(child).(*spacev1.ReconcileBotPurgeInstallationRequest)
	legacy.OperationId = uuid.NewString()
	legacy.BotId = uuid.NewString()
	legacy.InstallationId = uuid.NewString()
	legacy.NativeCancellation.ProfileId = actor.String()
	legacy.NativeCancellation.InstallOperationId = uuid.NewString()
	legacy.NativeCancellation.InstallationIntent.OperationId = uuid.NewString()
	legacy.NativeCancellation.InstallationIntent.BotId = legacy.BotId
	legacy.NativeCancellation.InstallationIntent.InstallationId = legacy.InstallationId
	legacy.NativeCancellation.InstallationIntent.ActorProfileId = actor.String()
	_, err = childCall(legacy)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "P3 cannot invent Bot provenance for an unrelated pre-journal source")
	type outcome struct {
		receipt *spacev1.BotPurgeInstallationReceipt
		err     error
	}
	outcomes := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() { r, e := childCall(child); outcomes <- outcome{r, e} }()
	}
	first, second := <-outcomes, <-outcomes
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.True(t, proto.Equal(first.receipt, second.receipt), "concurrent FIRST and exact replay converge to one own result")
	receipt := first.receipt
	require.Equal(t, spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_CANCELED, receipt.Disposition)
	require.Equal(t, child.OperationId, receipt.OperationId)
	replay, err := signedNativeP3ChildCaller(t, serving)(child)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replay))
	var settled bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.space_members WHERE space_id=$1 AND profile_id=$2) AND EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE install_operation_id=$3 AND state='CANCELED') AND EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children WHERE operation_id=$4) AND NOT EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=$1 AND participant_id=9 AND request_kind='PURGE' AND progress='COMPLETE') AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, space.ID, profile, add.InstallationIntent.OperationId, child.OperationId).Scan(&settled))
	require.True(t, settled, "native effect and typed result commit before Bot receipt, no aggregate cycle")
	settledChild := proto.Clone(child).(*spacev1.ReconcileBotPurgeInstallationRequest)
	settledChild.OperationId = uuid.NewString()
	adopted, err := childCall(settledChild)
	require.NoError(t, err)
	require.Equal(t, spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_ALREADY_CANCELED, adopted.Disposition)
	require.NotEqual(t, receipt.ReceiptId, adopted.ReceiptId, "distinct own P3 child records its exact already-canceled source; no second native removal")
	_, err = signedNativeP3ChildCaller(t, admin)(child)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "owner installer credentials are not a serving historical reader")
	_, err = db.Pool.Exec(ctx, `ALTER TABLE native_membership_guard.bot_purge_children DISABLE TRIGGER bot_purge49_immutable`)
	require.NoError(t, err)
	_, err = childCall(child)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "saved result cannot bypass current exact catalog qualification")
	_, err = db.Pool.Exec(ctx, `ALTER TABLE native_membership_guard.bot_purge_children ENABLE TRIGGER bot_purge49_immutable`)
	require.NoError(t, err)
	changed := proto.Clone(child).(*spacev1.ReconcileBotPurgeInstallationRequest)
	changed.NativeCancellation.InstallationIntent.OperationId = uuid.NewString()
	_, err = signedNativeP3ChildCaller(t, serving)(changed)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict, "saved operation cannot be rebound to another cancellation")
	// All actual current11 participant receipts are accepted by the coordinator;
	// no child result substitutes for the Bot participant's own typed receipt.
	for _, participant := range lifecycleTestParticipants[1:] {
		_, err = serving.RecordLifecyclePurgeReceipt(ctx, lifecyclePurgeReceipt(t, space.ID, operation, participant, 2, aggregate.Snapshot().PurgeDecidedAt))
		require.NoError(t, err)
	}
	// Freeze only this test PostgreSQL process clock before the real owner
	// samples FIRST aggregate completion. No product timestamps are rewritten.
	setNativeTerminalDBTime(t, admin, time.Now().UTC().Add(2*time.Second).Truncate(time.Second))
	waitQueued := func(pid int32, count int) {
		t.Helper()
		var queued int
		for until := time.Now().Add(3 * time.Second); time.Now().Before(until); {
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity w WHERE $1=ANY(pg_blocking_pids(w.pid)) OR EXISTS(
 SELECT 1 FROM pg_stat_activity b WHERE b.pid=ANY(pg_blocking_pids(w.pid)) AND $1=ANY(pg_blocking_pids(b.pid)))`, pid).Scan(&queued))
			if queued >= count {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		require.GreaterOrEqual(t, queued, count, "observe actual direct/queued Space-fence waiters")
	}
	finalizeBlocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer finalizeBlocker.Rollback(ctx)
	require.NoError(t, lockLifecycleSpace(ctx, finalizeBlocker, space.ID))
	var finalizePID int32
	require.NoError(t, finalizeBlocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&finalizePID))
	finalized := make(chan error, 1)
	go func() {
		_, err := serving.FinalizeLifecyclePurge(ctx, space.ID, testLifecycleHasher{})
		finalized <- err
	}()
	waitQueued(finalizePID, 1)
	replayed := make(chan outcome, 1)
	go func() { r, e := childCall(child); replayed <- outcome{r, e} }()
	waitQueued(finalizePID, 2)
	require.NoError(t, finalizeBlocker.Rollback(ctx))
	require.NoError(t, <-finalized)
	concurrentReplay := <-replayed
	require.NoError(t, concurrentReplay.err)
	require.True(t, proto.Equal(receipt, concurrentReplay.receipt), "queued replay survives actual finalization without live/source lookup")
	var deadline time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT parent_completed_at+interval '30 days' FROM native_membership_guard.bot_purge_children WHERE operation_id=$1`, child.OperationId).Scan(&deadline))
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1)
 AND NOT EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM public.space_bot_installation_cancellations WHERE space_id=$1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children WHERE operation_id=$2 AND parent_completed_at IS NOT NULL)`, space.ID, child.OperationId).Scan(&settled))
	require.True(t, settled, "child is retained independently of erased live Space/source journals")
	replay, err = signedNativeP3ChildCaller(t, serving)(child)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replay), "postDELETE replay returns exact original result without source lookup or reeffect")
	setNativeTerminalDBTime(t, admin, deadline.Add(-time.Second))
	replay, err = signedNativeP3ChildCaller(t, serving)(child)
	require.NoError(t, err)
	require.True(t, proto.Equal(receipt, replay))
	setNativeTerminalDBTime(t, admin, deadline)
	_, err = signedNativeP3ChildCaller(t, serving)(child)
	require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "strict DB deadline denies uncleared full bytes")
	rollback := errors.New("test rollback after actual fixed compact helper")
	err = nativeReadinessTx(ctx, serving.Pool, space.ID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT public.space_native_terminal_compact($1)`, space.ID); err != nil {
			return err
		}
		var remaining int
		if err := tx.QueryRow(ctx, `SELECT public.space_bot_purge_child_expire($1)`, space.ID).Scan(&remaining); err != nil {
			return err
		}
		require.Zero(t, remaining, "fixed parent helper already erased the child in this transaction")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	var compacted bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT evidence_compacted FROM public.space_lifecycle_aggregates WHERE space_id=$1`, space.ID).Scan(&compacted))
	require.False(t, compacted, "child erase and parent compaction roll back together")
	expiryBlocker, err := serving.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer expiryBlocker.Rollback(ctx)
	require.NoError(t, lockLifecycleSpace(ctx, expiryBlocker, space.ID))
	var expiryPID int32
	require.NoError(t, expiryBlocker.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&expiryPID))
	go func() { r, e := childCall(child); replayed <- outcome{r, e} }()
	waitQueued(expiryPID, 1)
	cleaned := make(chan error, 1)
	go func() { cleaned <- serving.CleanupLifecycleEvidence(ctx) }()
	waitQueued(expiryPID, 2)
	require.NoError(t, expiryBlocker.Rollback(ctx))
	expiredReplay := <-replayed
	require.ErrorIs(t, expiredReplay.err, ErrMemberAuthorityUnavailable, "concurrent deadline replay never discloses expired full evidence")
	require.NoError(t, <-cleaned)
	var remaining int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.bot_purge_children WHERE space_id=$1 OR operation_id=$2`, space.ID, child.OperationId).Scan(&remaining))
	require.Zero(t, remaining, "normal lifecycle cleanup must erase all child personal bytes, digests and replay mappings")
	_, err = signedNativeP3ChildCaller(t, serving)(child)
	require.Error(t, err, "expired child is unavailable; missing historical result cannot execute a new effect")
}
