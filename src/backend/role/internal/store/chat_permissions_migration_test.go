package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/role/permissions"
)

func TestSpaceChatCapabilitiesMigrationPreservesCustomRolesAndOverrides(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL Role capability backfill")
	}
	ctx, st := roleSourceStore(t)
	for _, migration := range []string{"000017_admin_role_management.up.sql", "000018_space_audit_outbox.up.sql", "000019_approve_join_requests.up.sql"} {
		_, err := st.Pool.Exec(ctx, role14SQL(t, migration))
		require.NoError(t, err)
	}
	space, owner, customSpace, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	_, err := st.Pool.Exec(ctx, `UPDATE roles SET permissions=1 WHERE space_id=$1`, space)
	require.NoError(t, err, "represent existing system roles without the new bits")
	custom, err := st.CreateCustomRole(ctx, customSpace, permissions.RoleAdmin, 3, 1, nil)
	require.NoError(t, err)
	require.NoError(t, st.SetChatOverride(ctx, chat, custom.ID, 1<<43, 1<<44))
	beforeOverrides := role14Rows(t, ctx, st.Pool, "chat_overrides")

	_, err = st.Pool.Exec(ctx, role14SQL(t, "000020_space_chat_permissions.up.sql"))
	require.NoError(t, err)
	roles, err := st.ListRoles(ctx, space)
	require.NoError(t, err)
	for _, role := range roles {
		want := uint64(1)
		switch role.Name {
		case permissions.RoleOwner, permissions.RoleAdmin:
			want |= 1<<43 | 1<<44
		case permissions.RoleMember, permissions.RoleModerator:
			want |= 1 << 44
		}
		require.Equal(t, want, role.PermissionsMask, "only the documented seed delta: %s", role.Name)
	}
	customAfter, err := st.GetRoleByID(ctx, custom.ID)
	require.NoError(t, err)
	require.Equal(t, uint64(3), customAfter.PermissionsMask, "same-name custom Admin has no seeded authority")
	require.Equal(t, beforeOverrides, role14Rows(t, ctx, st.Pool, "chat_overrides"))
}
