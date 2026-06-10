package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMaskFor_KnownPermissions(t *testing.T) {
	mask, err := MaskFor(SpaceManageInvites)
	require.NoError(t, err)
	require.NotZero(t, mask)
}

func TestSystemRoles_Hierarchy(t *testing.T) {
	roles, err := SystemRoles()
	require.NoError(t, err)
	require.Len(t, roles, 5)
	require.Equal(t, RoleOwner, roles[0].Name)
	require.Greater(t, roles[0].Position, roles[4].Position)
}

func TestHasPermission_MemberDefault(t *testing.T) {
	memberMask, err := memberDefaultMask()
	require.NoError(t, err)
	ok, err := HasPermission(memberMask, TextChatSendMessages)
	require.NoError(t, err)
	require.True(t, ok)
	guestMask, err := guestDefaultMask()
	require.NoError(t, err)
	ok, err = HasPermission(guestMask, TextChatSendMessages)
	require.NoError(t, err)
	require.False(t, ok)
}

func TestAllMask_Union(t *testing.T) {
	all, err := AllMask()
	require.NoError(t, err)
	send, err := MaskFor(TextChatSendMessages)
	require.NoError(t, err)
	require.NotZero(t, all & send)
}
