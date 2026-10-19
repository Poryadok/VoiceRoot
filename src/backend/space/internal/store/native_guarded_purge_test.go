package store

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeReadinessActualPurgeClosesTerminalFrame(t *testing.T) {
	testNativeReadinessActualPurgeClosesTerminalFrame(t, 44)
}

func TestNative45ActualPurgeClosesTerminalFrame(t *testing.T) {
	testNativeReadinessActualPurgeClosesTerminalFrame(t, 45)
}

func testNativeReadinessActualPurgeClosesTerminalFrame(t *testing.T, schema int) {
	for _, tc := range []struct {
		name           string
		empty, missing bool
	}{
		{"full current11", false, false}, {"empty cohort current11", true, false}, {"missing current11 evidence", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
			db.Run(true, "goto", "34")
			st := &SpaceStore{Pool: db.Pool}
			durable, _, _, actor, space, request, proof, hash := reserveLifecycleOperationFixture(t, st)
			op := uuid.MustParse(request.OperationId)
			_, err := durable.RecordLifecycleDeletionProofReceipt(ctx, actor, op, proof, hash)
			require.NoError(t, err)
			aggregate, err := st.BeginLifecycleFreeze(ctx, space, lifecycleManifestFixture())
			require.NoError(t, err)
			recordLifecycleFenceBarrier(t, aggregate, space, op, 1, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN)
			require.NoError(t, st.PersistLifecycle(ctx, aggregate))
			_, _, err = st.CompleteLifecycleSchedule(ctx, space)
			require.NoError(t, err)
			_, err = db.Pool.Exec(ctx, `UPDATE public.space_lifecycle_aggregates SET scheduled_at=scheduled_at-interval '8 days',purge_after=purge_after-interval '8 days' WHERE space_id=$1`, space)
			require.NoError(t, err)
			aggregate, err = st.DecideLifecycleRecovery(ctx, space, 2)
			require.NoError(t, err)
			recordLifecycleFenceBarrier(t, aggregate, space, op, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED)
			require.NoError(t, aggregate.BeginPurging())
			require.NoError(t, st.PersistLifecycle(ctx, aggregate))
			snapshot := aggregate.Snapshot()
			require.NoError(t, aggregate.RecordRoleRetirementReceipt(lifecycleRoleRetirementReceipt(t, space, op, 2, snapshot.PurgeDecidedAt)))
			participants := lifecycleTestParticipants[1:]
			if tc.missing {
				participants = participants[:len(participants)-1]
			}
			for _, id := range participants {
				require.NoError(t, aggregate.RecordPurgeReceipt(lifecyclePurgeReceipt(t, space, op, id, 2, snapshot.PurgeDecidedAt)))
			}
			require.NoError(t, st.PersistLifecycle(ctx, aggregate))
			if tc.empty {
				_, err = db.Pool.Exec(ctx, `DELETE FROM public.space_members WHERE space_id=$1`, space)
				require.NoError(t, err)
			}
			db.Run(true, "goto", strconv.Itoa(schema))
			if schema == 45 {
				_, err = st.TransitionNativeMembershipCatalog45(ctx)
				require.NoError(t, err)
			}
			require.NoError(t, nativeReadinessTx(ctx, db.Pool, space, func(tx pgx.Tx) error {
				return (&SpaceStore{Pool: db.Pool, tx: tx}).InitializeScopedNativeMembership(ctx, space)
			}), "authoritative PURGING UNKNOWN cutover is admin maintenance, not serving")
			serving := nativeReadinessLogin(t, ctx, db.Pool)
			actual := &SpaceStore{Pool: serving}
			completed, err := actual.FinalizeLifecyclePurge(ctx, space, testLifecycleHasher{})
			if tc.missing {
				require.Error(t, err)
				var untouched bool
				require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1) AND NOT EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=$1) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)`, space).Scan(&untouched))
				require.True(t, untouched)
				return
			}
			require.NoError(t, err, "actual transaction owner must capture before its first permanent-fence write")
			require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, completed.Phase())
			var terminal bool
			require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=$1 AND state='TERMINAL' AND terminal_xid IS NOT NULL AND octet_length(terminal_token_sha256)=32)
 AND NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.profiles WHERE space_id=$1)
 AND NOT EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1)
 AND public.space_membership_terminal_cleanup_allowed($1)`, space).Scan(&terminal))
			require.True(t, terminal, "same-TX terminal receipt closes and minimizes the actual capture")
			replayed, err := actual.FinalizeLifecyclePurge(ctx, space, testLifecycleHasher{})
			require.NoError(t, err)
			require.Equal(t, completed.Snapshot().DeletedEvent, replayed.Snapshot().DeletedEvent)
		})
	}
}
