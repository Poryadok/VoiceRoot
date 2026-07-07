package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSpaceModeration_PermissionBits documents spaces.md moderation permission names from role-service.md.
func TestSpaceModeration_PermissionBits(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"TEXT_CHAT_MANAGE_SETTINGS",
		"TEXT_CHAT_SET_SLOW_MODE",
		"MODERATION_TIMEOUT_MEMBERS",
	} {
		mask, err := MaskFor(name)
		require.NoError(t, err, "permission %q must be registered", name)
		require.NotZero(t, mask)
	}
}

// TestSpaceModeration_ModeratorRole_includesModerationPerms documents default Moderator role grants.
func TestSpaceModeration_ModeratorRole_includesModerationPerms(t *testing.T) {
	roles, err := SystemRoles()
	require.NoError(t, err)
	var modMask uint64
	for _, r := range roles {
		if r.Name == RoleModerator {
			modMask = r.Mask
		}
	}
	require.NotZero(t, modMask)

	for _, perm := range []string{
		MemberKick,
		MemberBan,
		"MODERATION_TIMEOUT_MEMBERS",
		"TEXT_CHAT_SET_SLOW_MODE",
	} {
		ok, err := HasPermission(modMask, perm)
		require.NoError(t, err)
		require.True(t, ok, "moderator must have %s", perm)
	}
}

// TestSpaceModeration_MemberRole_excludesModerationPerms documents Member cannot moderate by default.
func TestSpaceModeration_MemberRole_excludesModerationPerms(t *testing.T) {
	memberMask, err := memberDefaultMask()
	require.NoError(t, err)

	for _, perm := range []string{
		MemberKick,
		MemberBan,
		"MODERATION_TIMEOUT_MEMBERS",
		"TEXT_CHAT_SET_SLOW_MODE",
	} {
		ok, err := HasPermission(memberMask, perm)
		require.NoError(t, err)
		require.False(t, ok, "member must not have %s by default", perm)
	}
}
