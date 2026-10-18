package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type admissionCandidateAccess interface {
	LookupMemberAdmission(context.Context, uuid.UUID) (*MemberAdmissionIntent, error)
	FindCurrent(context.Context, uuid.UUID, uuid.UUID) (*MemberAdmissionIntent, error)
	RetireCandidate(context.Context, uuid.UUID, uint64, MemberAdmissionRetirementReason) (*MemberAdmissionIntent, error)
}

func TestMemberAdmissionCandidateRetirementRequiresCurrentProof(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Candidate retirement", "", "public")
	require.NoError(t, err)
	invite, err := st.CreateInvite(ctx, CreateInviteInput{SpaceID: space.ID, CreatorProfileID: owner})
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	access, ok := any(st).(admissionCandidateAccess)
	require.True(t, ok, "target journals need owned exact candidate lookup and proven retirement, not changed Prepare input")
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: uuid.New(), AccountID: uuid.New(), InviteID: &invite.ID}
	first, err := st.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	found, err := access.FindCurrent(ctx, in.SpaceID, in.ProfileID)
	require.NoError(t, err)
	require.Equal(t, first, found)
	_, err = access.RetireCandidate(ctx, in.OperationID, first.Generation, MemberAdmissionRetireInviteInvalid)
	require.ErrorIs(t, err, ErrMemberAdmissionConflict, "still-valid invite is not a retirement proof")
	_, err = pool.Exec(ctx, `UPDATE invites SET expires_at=clock_timestamp()-interval '1 second' WHERE id=$1`, invite.ID)
	require.NoError(t, err)
	_, err = access.RetireCandidate(ctx, in.OperationID, first.Generation+1, MemberAdmissionRetireInviteInvalid)
	require.ErrorIs(t, err, ErrMemberAdmissionStale, "wrong generation cannot retire even a known-expired invite")
	retired, err := access.RetireCandidate(ctx, in.OperationID, first.Generation, MemberAdmissionRetireInviteInvalid)
	require.NoError(t, err)
	require.Equal(t, "RETIRED", retired.State)
	_, err = access.RetireCandidate(ctx, in.OperationID, first.Generation, MemberAdmissionRetireInviteInvalid)
	require.NoError(t, err, "retirement replays without advancing another floor or cleanup")
	var cleanups int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_member_role_cleanup WHERE space_id=$1 AND profile_id=$2`, in.SpaceID, in.ProfileID).Scan(&cleanups))
	require.Equal(t, 1, cleanups, "candidate retires before Role READY and queues exactly one removal")
	_, err = access.FindCurrent(ctx, in.SpaceID, in.ProfileID)
	require.Error(t, err)
	lookup, err := access.LookupMemberAdmission(ctx, in.OperationID)
	require.NoError(t, err)
	require.Equal(t, "RETIRED", lookup.State)
	other := in
	other.OperationID, other.InviteID = uuid.New(), nil
	next, err := st.PrepareMemberAdmission(ctx, other)
	require.NoError(t, err)
	require.Greater(t, next.Generation, first.Generation)
	_, err = access.RetireCandidate(ctx, in.OperationID, first.Generation, MemberAdmissionRetireInviteInvalid)
	require.NoError(t, err)
	found, err = access.FindCurrent(ctx, in.SpaceID, in.ProfileID)
	require.NoError(t, err)
	require.Equal(t, next, found, "old retirement replay cannot affect a later invitation-bound attempt")
}
