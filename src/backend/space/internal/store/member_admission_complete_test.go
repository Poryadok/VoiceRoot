package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/memberadmission"
	"voice/backend/space/internal/memberadmissionrole"
)

type completionEnsureRPC func(context.Context, *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error)

func (f completionEnsureRPC) EnsureDefaultMemberRole(ctx context.Context, r *rolev1.EnsureDefaultMemberRoleRequest, _ ...grpc.CallOption) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
	return f(ctx, r)
}

func TestMemberAdmissionCompleteResumesFrozenRoleAndReplaysWithoutOwners(t *testing.T) {
	ctx := context.Background()
	st := nativeCreatorFixture(t)
	applyJoinAttemptMigration(t, st)
	applyJoinOperationMigration(t, st)
	i, selected, _ := prepareContinuationFixture(t, st, false, true)
	now := time.Now()
	subjects := continuationClient(t, &now, "regular")
	calls := 0
	var historicalRole uuid.UUID
	roles := &memberadmissionrole.Client{Issuer: subjects.Issuer, Clock: func() time.Time { return now }, Role: completionEnsureRPC(func(ctx context.Context, r *rolev1.EnsureDefaultMemberRoleRequest) (*rolev1.EnsureDefaultMemberRoleResponse, error) {
		calls++
		require.Equal(t, i.Input.OperationID.String(), r.AdmissionOperationId)
		require.Equal(t, i.Generation, r.MembershipGeneration)
		// This lock must be immediately acquirable: Role is never called under
		// Space's mutation fence. The production Role receiver has its own DB.
		tx, err := st.Pool.Begin(ctx)
		require.NoError(t, err)
		require.NoError(t, lockAndCheckOwnershipScope(ctx, tx, []uuid.UUID{i.Input.SpaceID}))
		require.NoError(t, tx.Rollback(ctx))
		if historicalRole == uuid.Nil {
			historicalRole = selected
		}
		binding := i.EnsureBinding()
		digest, err := memberadmission.EnsureReceiptFingerprint(binding, historicalRole.String())
		require.NoError(t, err)
		return &rolev1.EnsureDefaultMemberRoleResponse{Receipt: &rolev1.DefaultMemberRoleReceipt{ProtocolVersion: 1, SpaceId: binding.SpaceID, ProfileId: binding.ProfileID, AdmissionOperationId: binding.AdmissionOperationID, MembershipGeneration: binding.MembershipGeneration, AdmissionBindingSha256: binding.AdmissionBindingSHA256, RoleId: historicalRole.String(), ReceiptSha256: digest[:]}}, nil
	})}
	_, err := st.CompleteInteractiveMemberAdmission(ctx, i.Input.OperationID, roles, subjects, nil, func(context.Context, *SpaceStore, *MemberAdmissionIntent) error {
		now = now.Add(5 * time.Second)
		return nil
	})
	require.ErrorIs(t, err, ErrJoinAttemptUnavailable)
	require.Equal(t, 1, calls)
	selected = uuid.New() // Current configured default changed after inert Ensure.
	member, err := st.CompleteInteractiveMemberAdmission(ctx, i.Input.OperationID, roles, subjects, nil, nil)
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	var savedRole uuid.UUID
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT role_id FROM space_member_admission_intents WHERE operation_id=$1`, i.Input.OperationID).Scan(&savedRole))
	require.Equal(t, historicalRole, savedRole, "recovery must use original Role-selected receipt, not a new default")
	require.NoError(t, st.RemoveMember(ctx, i.Input.SpaceID, i.Input.ProfileID))
	oldOutcome, err := st.CompleteInteractiveMemberAdmission(ctx, i.Input.OperationID, nil, nil, nil, func(context.Context, *SpaceStore, *MemberAdmissionIntent) error {
		t.Fatal("completed coordinator replay ran hook")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, member, oldOutcome)
	require.Equal(t, 2, calls)
	active, err := st.IsSpaceMember(ctx, i.Input.SpaceID, i.Input.ProfileID)
	require.NoError(t, err)
	require.False(t, active, "historical result must not recreate membership after leave")
}
