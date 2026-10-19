package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestProtectedReportReadRequiresRealNativeCutover(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	admin := &SpaceStore{Pool: db.Pool}
	create, _, viewer := signedNativeCreatorCurrent(t, admin, &spacev1.CreateSpaceRequest{Name: "protected native reader", Visibility: "private"})
	space, err := create()
	require.NoError(t, err)
	chat := uuid.New()
	node, err := admin.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chat})
	require.NoError(t, err)
	db.Run(true, "up")
	var schemaVersion int
	var dirty bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT version,dirty FROM public.schema_migrations`).Scan(&schemaVersion, &dirty))
	require.Equal(t, 44, schemaVersion)
	require.False(t, dirty)
	login := nativeReadinessLogin(t, ctx, db.Pool)
	serving := &SpaceStore{Pool: login}
	facts, err := serving.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.NoError(t, err, "real44 native head/source/tree is the positive precondition, not a fake provider")
	require.True(t, facts.HistoryKnown)
	_, err = serving.ReadProtectedSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "valid positioning facts alone cannot substitute for actual completed cutover")
	require.NoError(t, nativeReadinessTx(ctx, db.Pool, space.ID, func(tx pgx.Tx) error {
		return (&SpaceStore{Pool: db.Pool, tx: tx}).InitializeScopedNativeMembership(ctx, space.ID)
	}))
	cut, err := serving.ReadProtectedSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.NoError(t, err)
	require.True(t, cut.Eligible)
	require.False(t, cut.HistoryKnown)
	require.True(t, cut.HistoryVisibleSince.IsZero())
	require.Equal(t, node.ID, cut.TreeNodeID)
	require.Equal(t, space.ID, cut.SpaceID)
	require.Equal(t, viewer, cut.ProfileID)
	require.Equal(t, facts.SourceBinding, cut.SourceBinding, "UNKNOWN cutover does not rebind native ONE-source identity")
	require.NotEqual(t, facts.EpisodeID, cut.EpisodeID)
	require.Positive(t, cut.ValidityRemaining)
	_, err = serving.ReadProtectedSpaceReportReadContext(ctx, space.ID, uuid.New(), chat)
	require.ErrorIs(t, err, ErrSpaceReportReadDenied)
	_, err = serving.ReadProtectedSpaceReportReadContext(ctx, space.ID, viewer, uuid.New())
	require.ErrorIs(t, err, ErrSpaceReportReadDenied)
	_, err = serving.ReadProtectedSpaceReportReadContext(ctx, uuid.New(), viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable)
	_, err = admin.ReadProtectedSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "installer is never a serving login")
	old, err := login.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	_, err = (&SpaceStore{Pool: login, tx: old}).ReadProtectedSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "caller cannot lend a stale snapshot to the serving producer")
	require.NoError(t, old.Rollback(ctx))
	t.Run("final clock follows guarded head wait", func(t *testing.T) {
		raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		holder, err := db.Pool.Begin(raceCtx)
		require.NoError(t, err)
		defer holder.Rollback(ctx)
		var holderPID int32
		require.NoError(t, holder.QueryRow(raceCtx, `SELECT pg_backend_pid()`).Scan(&holderPID))
		var lockedRevision int64
		require.NoError(t, holder.QueryRow(raceCtx, `SELECT ledger_revision FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, space.ID, viewer).Scan(&lockedRevision))
		done := make(chan error, 1)
		go func() {
			result, readErr := serving.ReadProtectedSpaceReportReadContext(raceCtx, space.ID, viewer, chat)
			if result != nil {
				done <- errors.New("expired protected read returned facts")
				return
			}
			done <- readErr
		}()
		require.Eventually(t, func() bool {
			var blocked bool
			return db.Pool.QueryRow(raceCtx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity a WHERE a.datname=current_database() AND $1=ANY(pg_blocking_pids(a.pid)))`, holderPID).Scan(&blocked) == nil && blocked
		}, 3*time.Second, 5*time.Millisecond)
		_, err = holder.Exec(raceCtx, `SELECT pg_sleep(5.05)`)
		require.NoError(t, err)
		require.NoError(t, holder.Commit(raceCtx))
		require.ErrorIs(t, <-done, ErrEffectiveEpisodeUnavailable)
		after, readErr := serving.ReadProtectedSpaceReportReadContext(ctx, space.ID, viewer, chat)
		require.NoError(t, readErr)
		require.Equal(t, lockedRevision, after.LedgerRevision)
	})
	var revision int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT ledger_revision FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, viewer).Scan(&revision))
	require.Equal(t, cut.LedgerRevision, revision, "read does not repair or advance history")
}

func TestProtectedReportReadAfterActualCurrent11Purge(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	durable := requireDurableLifecycleStore(t, admin)
	account, actor := uuid.New(), uuid.New()
	target, err := admin.CreateSpace(ctx, actor, "R23 durable lifecycle", "", "private")
	require.NoError(t, err)
	space := target.ID
	chat := uuid.New()
	_, err = admin.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space, Kind: TreeKindTextChat, ChatID: &chat})
	require.NoError(t, err)
	request := lifecycleScheduleRequest(space, uuid.New())
	_, err = durable.ReserveLifecycleSchedule(ctx, account, actor, 7, request)
	require.NoError(t, err)
	_, confirmationHash, proofDigest := lifecycleScheduleRequestEvidence(t, request)
	proof, hash := lifecycleAuthReceiptEvidence(t, account, actor, space, uuid.MustParse(request.OperationId), 7, confirmationHash, proofDigest)
	op := uuid.MustParse(request.OperationId)
	_, err = durable.RecordLifecycleDeletionProofReceipt(ctx, actor, op, proof, hash)
	require.NoError(t, err)
	aggregate, err := admin.BeginLifecycleFreeze(ctx, space, lifecycleManifestFixture())
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, space, op, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
	require.NoError(t, admin.PersistLifecycle(ctx, aggregate))
	_, _, err = admin.CompleteLifecycleSchedule(ctx, space)
	require.NoError(t, err)
	_, err = db.Pool.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space)
	require.NoError(t, err)
	aggregate, err = admin.DecideLifecycleRecovery(ctx, space, 2)
	require.NoError(t, err)
	recordLifecycleFenceBarrier(t, aggregate, space, op, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
	require.NoError(t, aggregate.BeginPurging())
	require.NoError(t, admin.PersistLifecycle(ctx, aggregate))
	snapshot := aggregate.Snapshot()
	require.NoError(t, aggregate.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, space, op, 2, snapshot.PurgeDecidedAt)))
	require.Len(t, lifecycleTestParticipants, 11)
	for _, participant := range lifecycleTestParticipants[1:] {
		require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, space, op, participant, 2, snapshot.PurgeDecidedAt)))
	}
	require.NoError(t, admin.PersistLifecycle(ctx, aggregate))
	// Earlier lifecycle writers are intentionally prepared before44: this does
	// not claim that every production writer has already adopted the new frame.
	db.Run(true, "goto", "41")
	create, _, viewer := signedNativeCreatorCurrent(t, admin, &spacev1.CreateSpaceRequest{Name: "unrelated live control", Visibility: "private"})
	live, err := create()
	require.NoError(t, err)
	liveChat := uuid.New()
	_, err = admin.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: live.ID, Kind: TreeKindTextChat, ChatID: &liveChat})
	require.NoError(t, err)
	db.Run(true, "up")
	for _, id := range []uuid.UUID{space, live.ID} {
		require.NoError(t, nativeReadinessTx(ctx, db.Pool, id, func(tx pgx.Tx) error {
			return (&SpaceStore{Pool: db.Pool, tx: tx}).InitializeScopedNativeMembership(ctx, id)
		}))
	}
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	control, err := serving.ReadProtectedSpaceReportReadContext(ctx, live.ID, viewer, liveChat)
	require.NoError(t, err)
	_, err = serving.ReadProtectedSpaceReportReadContext(ctx, space, actor, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "COMPLETE does not turn PURGING into a serving grant")
	completed, err := serving.FinalizeLifecyclePurge(ctx, space, testLifecycleHasher{})
	require.NoError(t, err)
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, completed.Phase())
	result, err := serving.ReadProtectedSpaceReportReadContext(ctx, space, actor, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable)
	require.Nil(t, result)
	var erased bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND state='TERMINAL') AND NOT EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1) AND NOT EXISTS(SELECT 1 FROM public.space_tree_nodes WHERE space_id=$1) AND public.space_membership_terminal_cleanup_allowed($1)`, space).Scan(&erased))
	require.True(t, erased)
	after, err := serving.ReadProtectedSpaceReportReadContext(ctx, live.ID, viewer, liveChat)
	require.NoError(t, err)
	require.Equal(t, control.EpisodeID, after.EpisodeID)
	require.Equal(t, control.LedgerRevision, after.LedgerRevision)
	_, err = serving.ReadProtectedSpaceReportReadContext(ctx, live.ID, viewer, chat)
	require.ErrorIs(t, err, ErrSpaceReportReadDenied, "deleted foreign attachment cannot be reused through another live Space")
}
