package permissions

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAdmissionApprovalPermissionDefaultsAndDelegation(t *testing.T) {
	const name = "SPACE_APPROVE_JOIN_REQUESTS"
	bit, err := MaskFor(name)
	require.NoError(t, err)
	require.Equal(t, uint64(1)<<42, bit)
	roles, err := SystemRoles()
	require.NoError(t, err)
	for _, role := range roles {
		allowed, err := HasPermission(role.Mask, name)
		require.NoError(t, err)
		require.Equal(t, role.Name == RoleOwner || role.Name == RoleAdmin || role.Name == RoleModerator, allowed, role.Name)
	}
	allowed, err := HasPermission(bit, name)
	require.NoError(t, err)
	require.True(t, allowed, "custom roles may delegate just approval")
	invite, err := MaskFor(SpaceManageInvites)
	require.NoError(t, err)
	allowed, err = HasPermission(invite, name)
	require.NoError(t, err)
	require.False(t, allowed, "invite management is not approval")
}
