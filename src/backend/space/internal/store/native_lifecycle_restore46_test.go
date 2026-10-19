package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative46RestoreRetryAndLiveEventUseGuardedOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("requires actual private Linux PostgreSQL18/schema46")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	f := newLifecycleOutcomeFixtureWithStore(t, admin, len(lifecycleTestParticipants))
	_, _, err := admin.CompleteLifecycleSchedule(ctx, f.space)
	require.NoError(t, err)
	db.Run(true, "up")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, f.space), "actual administrative first UNKNOWN cutover while frozen")
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	var before, after string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM public.space_ordinary_member_heads h WHERE space_id=$1 AND profile_id=$2`, f.space, f.actor).Scan(&before))
	request := &spacev1.RestoreSpaceRequest{SpaceId: f.space.String(), OperationId: uuid.NewString()}
	aggregate, err := serving.ReserveLifecycleRestore(ctx, f.account, f.actor, 7, request)
	require.NoError(t, err, "existing owner-authenticated restore reservation must frame its native writes")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_RESTORE_DECIDED, aggregate.Phase())
	require.NoError(t, serving.DeferLifecycleAttempt(ctx, f.space), "background retry metadata is still a guarded source write")
	for _, participant := range lifecycleTestParticipants {
		_, err = serving.RecordLifecycleFenceReceipt(ctx, lifecycleFenceReceiptFixture(t, f.space, uuid.MustParse(f.request.OperationId), participant, 2, commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, lifecycleManifestFixture()))
		require.NoError(t, err)
	}
	aggregate, event, err := serving.CompleteLifecycleRestore(ctx, f.space)
	require.NoError(t, err, "restore outcome and LIVE snapshot must commit in one completed native frame")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_LIVE, aggregate.Phase())
	require.NoError(t, serving.MarkLifecycleEventDelivered(ctx, event), "actual restored event delivery while Space remains LIVE")
	saved, err := serving.ReplayLifecycleRestoreOutcome(ctx, f.account, f.actor, 7, request)
	require.NoError(t, err)
	require.NotNil(t, saved)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM public.space_ordinary_member_heads h WHERE space_id=$1 AND profile_id=$2`, f.space, f.actor).Scan(&after))
	require.Equal(t, before, after, "reversible freeze/restore does not rebind or republish native membership")
	observed, err := serving.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: f.space.String(), ProfileId: f.actor.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionSourceState_MEMBER_ADMISSION_SOURCE_STATE_ACTIVE, observed.Ordinary.State)
	var delivered bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_lifecycle_outbox WHERE event_id=$1 AND state='DELIVERED' AND delivered_at IS NOT NULL) AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, event.EventID).Scan(&delivered))
	require.True(t, delivered)
}
