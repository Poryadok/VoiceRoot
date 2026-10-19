package store

import (
	"context"
	"crypto/sha256"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNative46LifecycleReservationAndProofUseGuardedOwner(t *testing.T) {
	if testing.Short() {
		t.Skip("requires private Linux PostgreSQL18 and actual schema46")
	}
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixtureWithImage(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"), "postgres:18-alpine")
	db.Run(true, "goto", "34")
	admin := &SpaceStore{Pool: db.Pool}
	account, actor := uuid.New(), uuid.New()
	space, err := admin.CreateSpace(ctx, actor, "R23 durable lifecycle", "", "private")
	require.NoError(t, err)
	db.Run(true, "up")
	_, err = admin.TransitionNativeMembershipCatalog46(ctx)
	require.NoError(t, err)
	require.NoError(t, admin.InitializeNativeMembership(ctx, space.ID))
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	call := signedNativeBotCaller(t, serving)
	profile := uuid.NewString()
	add := &spacev1.AddBotMemberRequest{SpaceId: space.ID.String(), ProfileId: profile, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: uuid.NewString(), InstallationId: uuid.NewString(), SpaceId: space.ID.String(), ActorProfileId: profile, Generation: 1}}
	_, err = call(add)
	require.NoError(t, err)
	decision := uuid.NewString()
	rootSHA, scopeSHA := sha256.Sum256([]byte(decision)), sha256.Sum256([]byte(uuid.NewString()))
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: add.SpaceId, ProfileId: profile, InstallationIntent: proto.Clone(add.InstallationIntent).(*spacev1.BotInstallationIntent), InstallOperationId: add.InstallationIntent.OperationId}
	cancel.InstallationIntent.OperationId = uuid.NewString()
	removal := &spacev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), DecisionId: decision, DecisionSha256: rootSHA[:], Purpose: "global_delete", BotId: add.InstallationIntent.BotId, ActorProfileId: profile, SpaceId: add.SpaceId, NativeCancellation: cancel, FrozenScopeSha256: scopeSHA[:]}
	receipt, err := call(removal)
	require.NoError(t, err)
	require.True(t, receipt.GlobalNativeAddsFenced)
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
 AND EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts WHERE operation_id=$3)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)`, operation, proof, removal.OperationId).Scan(&complete))
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
	remaining := lifecycleTestParticipants[1:]
	for _, participant := range remaining[:len(remaining)-1] {
		_, err = serving.RecordLifecyclePurgeReceipt(ctx, lifecyclePurgeReceipt(t, space.ID, operation, participant, 2, aggregate.Snapshot().PurgeDecidedAt))
		require.NoError(t, err)
	}
	_, err = serving.FinalizeLifecyclePurge(ctx, space.ID, testLifecycleHasher{})
	require.Error(t, err, "one missing current participant must preserve actual global receipt and Space")
	var retained bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts WHERE operation_id=$2)
 AND NOT EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=$1)`, space.ID, removal.OperationId).Scan(&retained))
	require.True(t, retained)
	_, err = serving.RecordLifecyclePurgeReceipt(ctx, lifecyclePurgeReceipt(t, space.ID, operation, remaining[len(remaining)-1], 2, aggregate.Snapshot().PurgeDecidedAt))
	require.NoError(t, err)
	completed, err := serving.FinalizeLifecyclePurge(ctx, space.ID, testLifecycleHasher{})
	require.NoError(t, err, "actual full current11 terminal transition must clean the real native global receipt")
	require.Equal(t, spacev1.SpaceDeletionPhase_SPACE_DELETION_PHASE_PURGED, completed.Phase())
	var minimized bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts WHERE space_id=$1)
 AND EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=$2 AND decision_id=$3)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames)
 AND public.space_membership_terminal_cleanup_allowed($1)`, space.ID, removal.BotId, removal.DecisionId).Scan(&minimized))
	require.True(t, minimized)
}
