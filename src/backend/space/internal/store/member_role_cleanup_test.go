package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/memberadmission"
)

type memberRoleCleanupAccess interface {
	ListPendingMemberRoleRemovals(context.Context, int) ([]memberadmission.RemoveBinding, error)
	MarkMemberRoleRemovalCompleted(context.Context, memberadmission.RemoveBinding, []byte) error
}

func TestMemberRoleCleanupRequiresExactDurableReceiptAndCannotRebind(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	space, err := st.CreateSpace(ctx, uuid.New(), "Role cleanup", "", "public")
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	access, ok := any(st).(memberRoleCleanupAccess)
	require.True(t, ok, "a durable removal must be delivered and acknowledged without holding Space locks across Role RPC")
	in := MemberAdmissionInput{OperationID: uuid.New(), SpaceID: space.ID, ProfileID: uuid.New(), AccountID: uuid.New()}
	_, err = st.PrepareMemberAdmission(ctx, in)
	require.NoError(t, err)
	require.NoError(t, st.RemoveMember(ctx, space.ID, in.ProfileID))
	rows, err := access.ListPendingMemberRoleRemovals(ctx, 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	binding := rows[0]
	require.NoError(t, memberadmission.ValidateRemove(binding))
	require.Equal(t, space.ID.String(), binding.SpaceID)
	require.Equal(t, in.ProfileID.String(), binding.ProfileID)
	receipt, err := memberadmission.RemoveReceiptFingerprint(binding)
	require.NoError(t, err)
	require.ErrorIs(t, access.MarkMemberRoleRemovalCompleted(ctx, binding, make([]byte, 32)), ErrMemberAdmissionConflict)
	changed, err := memberRoleRemovalBinding(uuid.MustParse(binding.RemovalOperationID), uuid.MustParse(binding.SpaceID), uuid.MustParse(binding.ProfileID), binding.RemovalGenerationFloor+1)
	require.NoError(t, err)
	changedReceipt, err := memberadmission.RemoveReceiptFingerprint(changed)
	require.NoError(t, err)
	require.ErrorIs(t, access.MarkMemberRoleRemovalCompleted(ctx, changed, changedReceipt[:]), ErrMemberAdmissionConflict)
	require.NoError(t, access.MarkMemberRoleRemovalCompleted(ctx, binding, receipt[:]))
	var first string
	require.NoError(t, pool.QueryRow(ctx, `SELECT completed_at::text FROM space_member_role_cleanup WHERE operation_id=$1`, binding.RemovalOperationID).Scan(&first))
	require.NoError(t, access.MarkMemberRoleRemovalCompleted(ctx, binding, receipt[:]))
	var replay string
	require.NoError(t, pool.QueryRow(ctx, `SELECT completed_at::text FROM space_member_role_cleanup WHERE operation_id=$1`, binding.RemovalOperationID).Scan(&replay))
	require.Equal(t, first, replay)
	rows, err = access.ListPendingMemberRoleRemovals(ctx, 100)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = pool.Exec(ctx, `UPDATE space_member_role_cleanup SET removal_floor=removal_floor+1 WHERE operation_id=$1`, binding.RemovalOperationID)
	require.Error(t, err, "an acknowledged intent must not be rebound")
	_, err = pool.Exec(ctx, `DELETE FROM space_member_role_cleanup WHERE operation_id=$1`, binding.RemovalOperationID)
	require.Error(t, err, "ordinary removal cannot erase durable cleanup evidence")
}
