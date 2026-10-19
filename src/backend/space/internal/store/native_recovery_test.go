package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestNativeRecoveryRequiresExactFinalizedReceiptAndPreservesExistingEpisode(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	bootstrap := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "native-recovery", TemplateID: "guild-default"}
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, bootstrap)
	require.NoError(t, err)
	in := CommunityOwnerRecoveryInput{OperationID: uuid.New(), ApplicationID: bootstrap.ApplicationID, EnvironmentID: bootstrap.EnvironmentID, CorporationKey: bootstrap.CorporationKey, SpaceID: space.ID, PreviousOwnerAccountID: bootstrap.OwnerAccountID, PreviousOwnerProfileID: bootstrap.OwnerProfileID, ReplacementAccountID: uuid.New(), ReplacementProfileID: uuid.New(), ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSHA256: make([]byte, 32)}
	_, _, err = st.ReserveCommunityOwnerRecovery(ctx, in)
	require.NoError(t, err)
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, []byte("not a Role receipt"))
	require.ErrorIs(t, err, ErrCommunityOwnerRecoveryConflict)
	receipt := &rolev1.OwnershipTransferReceipt{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: in.SpaceID.String(), OperationId: in.OperationID.String(), OldOwnerProfileId: in.PreviousOwnerProfileID.String(), NewOwnerProfileId: in.ReplacementProfileID.String()}, State: rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, CurrentOwnerProfileId: in.ReplacementProfileID.String()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	require.NoError(t, err)
	firstReceipt := append([]byte(nil), raw...)
	for name, mutate := range map[string]func(*rolev1.OwnershipTransferReceipt){
		"prepared": func(r *rolev1.OwnershipTransferReceipt) {
			r.State = rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_PREPARED
		},
		"foreign operation": func(r *rolev1.OwnershipTransferReceipt) { r.Intent.OperationId = uuid.NewString() },
		"old owner":         func(r *rolev1.OwnershipTransferReceipt) { r.CurrentOwnerProfileId = in.PreviousOwnerProfileID.String() },
		"unknown intent":    func(r *rolev1.OwnershipTransferReceipt) { r.Intent.ProtoReflect().SetUnknown([]byte{0x78, 1}) },
	} {
		t.Run(name, func(t *testing.T) {
			bad := proto.Clone(receipt).(*rolev1.OwnershipTransferReceipt)
			mutate(bad)
			encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(bad)
			require.NoError(t, err)
			_, err = st.CompleteCommunityOwnerRecovery(ctx, in, encoded)
			require.ErrorIs(t, err, ErrCommunityOwnerRecoveryConflict)
		})
	}
	type completion struct {
		operation CommunityOwnerRecoveryOperation
		err       error
	}
	completed := make(chan completion, 2)
	for range 2 {
		go func() {
			op, err := st.CompleteCommunityOwnerRecovery(ctx, in, raw)
			completed <- completion{op, err}
		}()
	}
	for range 2 {
		result := <-completed
		require.NoError(t, result.err)
		require.EqualValues(t, 2, result.operation.ResultGeneration)
	}
	observed, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: in.ReplacementProfileID.String()})
	require.NoError(t, err)
	require.EqualValues(t, 1, observed.GetOrdinary().GetHeadGeneration())
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_RECOVERY, observed.GetOrdinary().GetPublicationKind())
	require.Empty(t, observed.GetOrdinary().GetRoleReceiptSha256())
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, raw)
	require.NoError(t, err)
	var count int
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	// Return ownership to the still-present original ordinary member. This must
	// retain its bootstrap episode, rather than mint a second native/default grant.
	back := in
	back.OperationID, back.PreviousOwnerAccountID, back.PreviousOwnerProfileID = uuid.New(), in.ReplacementAccountID, in.ReplacementProfileID
	back.ReplacementAccountID, back.ReplacementProfileID, back.ExpectedGeneration = bootstrap.OwnerAccountID, bootstrap.OwnerProfileID, 2
	_, _, err = st.ReserveCommunityOwnerRecovery(ctx, back)
	require.NoError(t, err)
	receipt.Intent.OperationId, receipt.Intent.OldOwnerProfileId, receipt.Intent.NewOwnerProfileId = back.OperationID.String(), back.PreviousOwnerProfileID.String(), back.ReplacementProfileID.String()
	receipt.CurrentOwnerProfileId = back.ReplacementProfileID.String()
	raw, err = proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	require.NoError(t, err)
	_, err = st.CompleteCommunityOwnerRecovery(ctx, back, raw)
	require.NoError(t, err)
	original, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: bootstrap.OwnerProfileID.String()})
	require.NoError(t, err)
	require.Equal(t, bootstrap.OperationID.String(), original.GetOrdinary().GetPublishedBinding().GetOrdinary().GetPublicationIdentity())
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE, original.GetOrdinary().GetPublicationKind())
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, st.RemoveMember(ctx, space.ID, in.ReplacementProfileID))
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, firstReceipt)
	require.NoError(t, err)
	member, err := st.IsSpaceMember(ctx, space.ID, in.ReplacementProfileID)
	require.NoError(t, err)
	require.False(t, member, "completed recovery replay cannot resurrect a later removed member")
	_, err = st.Pool.Exec(ctx, `UPDATE spaces SET visibility='public' WHERE id=$1`, space.ID)
	require.NoError(t, err)
	candidate, err := st.PrepareMemberAdmission(ctx, MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, AccountID: in.ReplacementAccountID, ProfileID: in.ReplacementProfileID})
	require.NoError(t, err)
	next := in
	next.OperationID, next.ExpectedGeneration = uuid.New(), 3
	_, _, err = st.ReserveCommunityOwnerRecovery(ctx, next)
	require.NoError(t, err)
	receipt.Intent.OperationId, receipt.Intent.OldOwnerProfileId, receipt.Intent.NewOwnerProfileId = next.OperationID.String(), next.PreviousOwnerProfileID.String(), next.ReplacementProfileID.String()
	receipt.CurrentOwnerProfileId = next.ReplacementProfileID.String()
	raw, err = proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	require.NoError(t, err)
	_, err = st.CompleteCommunityOwnerRecovery(ctx, next, raw)
	require.NoError(t, err)
	current, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: next.ReplacementProfileID.String()})
	require.NoError(t, err)
	require.EqualValues(t, candidate.Generation+2, current.GetOrdinary().GetHeadGeneration())
	require.EqualValues(t, candidate.Generation+1, current.GetOrdinary().GetRetiredThrough())
	var state string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT state FROM space_member_admission_intents WHERE operation_id=$1`, candidate.Input.OperationID).Scan(&state))
	require.Equal(t, "RETIRED", state)
	var cleanup bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM space_member_role_cleanup WHERE space_id=$1 AND profile_id=$2 AND removal_floor=$3)`, space.ID, next.ReplacementProfileID, candidate.Generation+1).Scan(&cleanup))
	require.True(t, cleanup, "recovery invalidates the obstructing inert Role candidate through its own floor")
}

func TestNativeRecoveryMemberFailureRollsBackOwnerProofAndOutcomeTogether(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	bootstrap := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "rollback-recovery", TemplateID: "guild-default"}
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, bootstrap)
	require.NoError(t, err)
	in := CommunityOwnerRecoveryInput{OperationID: uuid.New(), ApplicationID: bootstrap.ApplicationID, EnvironmentID: bootstrap.EnvironmentID, CorporationKey: bootstrap.CorporationKey, SpaceID: space.ID, PreviousOwnerAccountID: bootstrap.OwnerAccountID, PreviousOwnerProfileID: bootstrap.OwnerProfileID, ReplacementAccountID: uuid.New(), ReplacementProfileID: uuid.New(), ExpectedGeneration: 1, ReasonCode: "owner_lost", EvidenceSHA256: make([]byte, 32)}
	_, _, err = st.ReserveCommunityOwnerRecovery(ctx, in)
	require.NoError(t, err)
	receipt := &rolev1.OwnershipTransferReceipt{Intent: &rolev1.OwnershipTransferIntent{ProtocolVersion: 2, SpaceId: in.SpaceID.String(), OperationId: in.OperationID.String(), OldOwnerProfileId: in.PreviousOwnerProfileID.String(), NewOwnerProfileId: in.ReplacementProfileID.String()}, State: rolev1.OwnershipTransferState_OWNERSHIP_TRANSFER_STATE_FINALIZED, CurrentOwnerProfileId: in.ReplacementProfileID.String()}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(receipt)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `CREATE FUNCTION fail_recovery_member() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected recovery member failure'; END $$; CREATE TRIGGER fail_recovery_member BEFORE INSERT ON space_members FOR EACH ROW EXECUTE FUNCTION fail_recovery_member()`)
	require.NoError(t, err)
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, raw)
	require.Error(t, err)
	var unchanged bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT s.owner_profile_id=$2 AND s.member_count=1 AND a.owner_profile_id=$2 AND a.owner_generation=1 AND a.status='recovery_pending' AND o.status='pending' AND o.role_receipt IS NULL
	 AND NOT EXISTS(SELECT 1 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$4)
	 AND NOT EXISTS(SELECT 1 FROM space_native_member_publications WHERE operation_id=$3)
	 FROM spaces s JOIN community_owner_authority a ON a.space_id=s.id JOIN community_owner_recovery_operations o ON o.space_id=s.id WHERE s.id=$1 AND o.operation_id=$3`, space.ID, bootstrap.OwnerProfileID, in.OperationID, in.ReplacementProfileID).Scan(&unchanged))
	require.True(t, unchanged)
	_, err = st.Pool.Exec(ctx, `DROP TRIGGER fail_recovery_member ON space_members`)
	require.NoError(t, err)
	_, err = st.CompleteCommunityOwnerRecovery(ctx, in, raw)
	require.NoError(t, err, "exact recovery retry resumes after rolled-back local publication")
}
