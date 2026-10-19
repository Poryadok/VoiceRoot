package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

// Role receipt bytes exercise the existing local receipt contract only; this
// Store test does not replace the protected Role caller/authorization assembly.
func TestNative45CommunityRecoveryPreservesOrdinaryAndRetiresRoster(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	// Establish real existing source facts before the all-writer guard. This
	// tests recovery of an initialized cohort; BORN creation has its own oracle.
	db.Run(true, "goto", "39")
	st := &SpaceStore{Pool: db.Pool}
	bootstrap := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "native-recovery", TemplateID: "guild-default"}
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, bootstrap)
	require.NoError(t, err)
	member := uuid.New()
	_, err = st.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: uuid.New(), ApplicationID: bootstrap.ApplicationID, EnvironmentID: bootstrap.EnvironmentID, SpaceID: space.ID, CorporationKey: bootstrap.CorporationKey, OwnerGeneration: 1, SourceRevision: 1, SnapshotSHA256: make([]byte, 32), LeaseExpiresAt: time.Now().Add(110 * time.Second), ProfileIDs: []uuid.UUID{member}})
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, st.InitializeNativeMembership(ctx, space.ID))
	db.Run(true, "goto", "45")
	_, err = st.TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err)
	st = &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	var originalOwner string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM public.space_ordinary_member_heads h WHERE space_id=$1 AND profile_id=$2`, space.ID, bootstrap.OwnerProfileID).Scan(&originalOwner))
	in := CommunityOwnerRecoveryInput{OperationID: uuid.New(), ApplicationID: bootstrap.ApplicationID, EnvironmentID: bootstrap.EnvironmentID, CorporationKey: bootstrap.CorporationKey, SpaceID: space.ID, PreviousOwnerAccountID: bootstrap.OwnerAccountID, PreviousOwnerProfileID: bootstrap.OwnerProfileID, ReplacementAccountID: uuid.New(), ReplacementProfileID: uuid.New(), ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSHA256: make([]byte, 32)}
	reserved, fresh, err := st.ReserveCommunityOwnerRecovery(ctx, in)
	require.NoError(t, err, "native owner recovery reservation must capture before journal/status source writes")
	require.True(t, fresh)
	require.Equal(t, "pending", reserved.Status)
	var retired bool
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 AND state='RETIRED' AND generation=retired_through)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$1)`, space.ID, member).Scan(&retired))
	require.True(t, retired, "recovery freeze is a real community authority loss, not a late lease extension")
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 1, count, "old owner's ordinary source survives while the roster loses eligibility")
	_, fresh, err = st.ReserveCommunityOwnerRecovery(ctx, in)
	require.NoError(t, err)
	require.False(t, fresh)
	receipt := &rolev1.OwnershipTransferReceipt{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: space.ID.String(), OperationId: in.OperationID.String(), OldOwnerProfileId: in.PreviousOwnerProfileID.String(), NewOwnerProfileId: in.ReplacementProfileID.String()}, State: rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, CurrentOwnerProfileId: in.ReplacementProfileID.String()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	require.NoError(t, err)
	wrong := proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt)
	wrong.Intent.OperationId = uuid.NewString()
	wrongBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(wrong)
	require.NoError(t, err)
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, wrongBytes)
	require.ErrorIs(t, err, ErrCommunityOwnerRecoveryConflict, "another operation's finalized bytes cannot authorize completion")
	completed, err := st.CompleteCommunityOwnerRecovery(ctx, in, raw)
	require.NoError(t, err, "exact finalized recovery must publish nativekind5 and finish the captured full cohort")
	require.Equal(t, "succeeded", completed.Status)
	require.EqualValues(t, 2, completed.ResultGeneration)
	var afterOwner string
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT to_jsonb(h)::text FROM public.space_ordinary_member_heads h WHERE space_id=$1 AND profile_id=$2`, space.ID, bootstrap.OwnerProfileID).Scan(&afterOwner))
	require.Equal(t, originalOwner, afterOwner, "former Owner keeps its exact member episode without implicit default-role grant")
	facts, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: in.ReplacementProfileID.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY, facts.Ordinary.PublicationKind)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	var snapshot, replayed string
	const state = `SELECT jsonb_build_object('heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM public.space_effective_member_heads h WHERE space_id=$1),'registry',(SELECT to_jsonb(r) FROM native_membership_guard.spaces r WHERE space_id=$1))::text`
	require.NoError(t, db.Pool.QueryRow(ctx, state, space.ID).Scan(&snapshot))
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, raw)
	require.NoError(t, err)
	require.NoError(t, db.Pool.QueryRow(ctx, state, space.ID).Scan(&replayed))
	require.Equal(t, snapshot, replayed, "completed recovery replay never republishes or renews history")
	t.Run("waiting final history SQL rolls back reservation", func(t *testing.T) {
		next := in
		next.OperationID, next.PreviousOwnerAccountID, next.PreviousOwnerProfileID = uuid.New(), in.ReplacementAccountID, in.ReplacementProfileID
		next.ReplacementAccountID, next.ReplacementProfileID, next.ExpectedGeneration = uuid.New(), uuid.New(), 2
		var before, after string
		const allState = `SELECT jsonb_build_object('heads',(SELECT jsonb_agg(to_jsonb(h) ORDER BY profile_id) FROM public.space_effective_member_heads h WHERE space_id=$1),
 'authority',(SELECT to_jsonb(a) FROM public.community_owner_authority a WHERE space_id=$1),
 'registry',(SELECT to_jsonb(r) FROM native_membership_guard.spaces r WHERE space_id=$1))::text`
		require.NoError(t, db.Pool.QueryRow(ctx, allState, space.ID).Scan(&before))
		blocker, err := db.Pool.Begin(ctx)
		require.NoError(t, err)
		defer blocker.Rollback(ctx)
		_, err = blocker.Exec(ctx, `LOCK TABLE public.space_effective_member_heads IN SHARE MODE`)
		require.NoError(t, err)
		result := make(chan error, 1)
		go func() { _, _, err := st.ReserveCommunityOwnerRecovery(ctx, next); result <- err }()
		require.Eventually(t, func() bool {
			var waiting bool
			err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE relation='public.space_effective_member_heads'::regclass AND mode='RowExclusiveLock' AND NOT granted)`).Scan(&waiting)
			return err == nil && waiting
		}, 4*time.Second, 10*time.Millisecond, "actual final history write waits after reservation/status writes")
		time.Sleep(5050 * time.Millisecond)
		require.NoError(t, blocker.Commit(ctx))
		require.ErrorIs(t, <-result, ErrEffectiveEpisodeUnavailable)
		require.NoError(t, db.Pool.QueryRow(ctx, allState, space.ID).Scan(&after))
		require.Equal(t, before, after)
		var clean bool
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM public.community_owner_recovery_operations WHERE operation_id=$1)
 AND NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=$2)`, next.OperationID, space.ID).Scan(&clean))
		require.True(t, clean)
	})
}
