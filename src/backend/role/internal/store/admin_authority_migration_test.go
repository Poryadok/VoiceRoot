package store

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/permissions"
)

func TestAdminAuthorityMigrationPreservesRolesOverridesAndRevision(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL authority catalog and migration")
	}
	ctx, st := roleSourceStore(t)
	space, owner, other, customSpace, room := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, st.BootstrapSpaceRoles(ctx, space, owner))
	require.NoError(t, st.BootstrapSpaceRoles(ctx, other, uuid.New()))
	legacy, err := st.RoleIDByName(ctx, space, permissions.RoleAdmin)
	require.NoError(t, err)
	already, err := st.RoleIDByName(ctx, other, permissions.RoleAdmin)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE roles SET permissions=2 WHERE id=$1;`, legacy)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `UPDATE roles SET permissions=6 WHERE id=$1`, already)
	require.NoError(t, err)
	custom, err := st.CreateCustomRole(ctx, customSpace, permissions.RoleAdmin, 2, 1, nil)
	require.NoError(t, err)
	require.NoError(t, st.SetVoiceRoomOverride(ctx, room, legacy, 4, 4))
	customBefore := role14Rows(t, ctx, st.Pool, "voice_room_overrides")
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()}
	before, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000017_admin_role_management.up.sql"))
	require.NoError(t, err)
	for id, want := range map[uuid.UUID]uint64{legacy: 6, already: 6, custom.ID: 2} {
		row, err := st.GetRoleByID(ctx, id)
		require.NoError(t, err)
		require.Equal(t, want, row.PermissionsMask)
	}
	require.Equal(t, customBefore, role14Rows(t, ctx, st.Pool, "voice_room_overrides"))
	_, err = st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=17,dirty=false`)
	require.NoError(t, err)
	after, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err, "data-only upgrade must keep complete authoritative reads available")
	require.Equal(t, before+1, after, "only the changed Admin row emits a policy invalidation")
	rowsAfter := role14Rows(t, ctx, st.Pool, "roles")
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000017_admin_role_management.up.sql"))
	require.NoError(t, err)
	require.Equal(t, rowsAfter, role14Rows(t, ctx, st.Pool, "roles"), "repeated UP must not rewrite data/timestamps")
	revision, err := st.ReadAuthorityRevision(ctx, scope)
	require.NoError(t, err)
	require.Equal(t, after, revision)
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000017_admin_role_management.down.sql"))
	require.Error(t, err, "rollback cannot identify and revoke only previously absent grants")
	require.Equal(t, rowsAfter, role14Rows(t, ctx, st.Pool, "roles"))
}
