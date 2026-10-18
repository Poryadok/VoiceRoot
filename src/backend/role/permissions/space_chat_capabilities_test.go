package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Parent BE-A2-CHAT-MANAGEMENT-PERMISSIONS c5 keeps invitation and official
// publication distinct from ordinary sending and settings management.
func TestSpaceChatCapabilityRegistryAndSeedDefaults(t *testing.T) {
	post, err := MaskFor("TEXT_CHAT_POST_AS_CHAT")
	require.NoError(t, err)
	add, err := MaskFor("TEXT_CHAT_ADD_MEMBERS")
	require.NoError(t, err)
	require.Equal(t, uint64(1)<<43, post)
	require.Equal(t, uint64(1)<<44, add)
	require.Zero(t, post&add)
	for _, name := range []string{TextChatSendMessages, TextChatManageSettings, SpaceManageInvites} {
		bit, err := MaskFor(name)
		require.NoError(t, err)
		require.Zero(t, bit&(post|add), "separate capability: %s", name)
	}
	roles, err := SystemRoles()
	require.NoError(t, err)
	for _, role := range roles {
		switch role.Name {
		case RoleOwner, RoleAdmin:
			require.Equal(t, post|add, role.Mask&(post|add), role.Name)
		case RoleMember, RoleModerator:
			require.Equal(t, add, role.Mask&(post|add), role.Name)
		case RoleGuest:
			require.Zero(t, role.Mask&(post|add), "no new blanket guest power")
		default:
			t.Fatalf("unexpected seeded role %q", role.Name)
		}
	}
}
