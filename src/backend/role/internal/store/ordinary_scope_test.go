package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/role/permissions"
)

type ordinaryScopeFixture struct {
	s                                                     *RoleStore
	space, owner, emptyActor, custom, member, chat, voice uuid.UUID
}

func startOrdinaryScopeDB(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := StartRoleDBForStoreTest(t, ctx)
	t.Cleanup(pool.Close)
	ApplyRoleMigrationsForStoreTest(t, ctx, pool)
	// R15 owns the production migration. Until integrated, these columns provide
	// its agreed ordinary-fence surface without changing production migration lists.
	_, err := pool.Exec(ctx, `
 CREATE TABLE IF NOT EXISTS ownership_transfer_v2 (
 operation_id UUID PRIMARY KEY, space_id UUID NOT NULL, protocol_version INTEGER NOT NULL,
 old_owner_profile_id UUID NOT NULL, new_owner_profile_id UUID NOT NULL,
 intent_bytes BYTEA NOT NULL, intent_hash BYTEA NOT NULL, state TEXT NOT NULL,
 prepare_request_hash BYTEA);
 CREATE TABLE IF NOT EXISTS role_space_lifecycle (space_id UUID PRIMARY KEY, retired_at TIMESTAMPTZ);`)
	require.NoError(t, err)
	return pool
}

func newOrdinaryScopeFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) ordinaryScopeFixture {
	t.Helper()
	f := ordinaryScopeFixture{s: &RoleStore{Pool: pool}, space: uuid.New(), owner: uuid.New(), emptyActor: uuid.New(), chat: uuid.New(), voice: uuid.New()}
	require.NoError(t, f.s.BootstrapSpaceRoles(ctx, f.space, f.owner))
	var err error
	f.member, err = f.s.RoleIDByName(ctx, f.space, permissions.RoleMember)
	require.NoError(t, err)
	custom, err := f.s.CreateCustomRole(ctx, f.space, "fixture custom", 1, 1, &f.owner)
	require.NoError(t, err)
	f.custom = custom.ID
	require.NoError(t, f.s.AssignMemberRole(ctx, f.space, f.owner, f.custom, f.owner))
	require.NoError(t, f.s.SetChatOverride(ctx, f.chat, f.custom, 1, 2))
	require.NoError(t, f.s.SetVoiceRoomOverride(ctx, f.voice, f.custom, 1, 2))
	return f
}

func prepareOrdinaryScope(t *testing.T, ctx context.Context, pool *pgxpool.Pool, f ordinaryScopeFixture) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO ownership_transfer_v2
 (operation_id,space_id,protocol_version,old_owner_profile_id,new_owner_profile_id,intent_bytes,intent_hash,state,prepare_request_hash)
 VALUES ($1,$2,2,$3,$4,$5,$6,'prepared',$6)`, uuid.New(), f.space, f.owner, uuid.New(), []byte("fixture intent"), make([]byte, 32))
	require.NoError(t, err)
}

func ordinaryRowsSnapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) string {
	t.Helper()
	var snapshot string
	require.NoError(t, pool.QueryRow(ctx, `SELECT jsonb_build_object(
 'roles', (SELECT jsonb_agg(to_jsonb(r) ORDER BY id) FROM roles r),
 'members', (SELECT jsonb_agg(to_jsonb(m) ORDER BY space_id,profile_id,role_id) FROM member_roles m),
 'chat', (SELECT jsonb_agg(to_jsonb(c) ORDER BY chat_id,role_id) FROM chat_overrides c),
 'voice', (SELECT jsonb_agg(to_jsonb(v) ORDER BY voice_room_id,role_id) FROM voice_room_overrides v)
 )::text`).Scan(&snapshot))
	return snapshot
}

func TestOrdinaryStorePreparedFence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := startOrdinaryScopeDB(t, ctx)
	// Each exported ordinary helper is invoked directly. Returning the fence error
	// after a write or alongside usable authority is also a failure.
	for _, tc := range []struct {
		name string
		call func(ordinaryScopeFixture) (any, error)
	}{
		{"BootstrapSystemRoles", func(f ordinaryScopeFixture) (any, error) { return nil, f.s.BootstrapSystemRoles(ctx, f.space) }},
		{"BootstrapSpaceRoles", func(f ordinaryScopeFixture) (any, error) { return nil, f.s.BootstrapSpaceRoles(ctx, f.space, f.owner) }},
		{"BootstrapSpaceRolesWithCreatedSystemRoles", func(f ordinaryScopeFixture) (any, error) {
			return f.s.BootstrapSpaceRolesWithCreatedSystemRoles(ctx, f.space, f.owner)
		}},
		{"ListRoles", func(f ordinaryScopeFixture) (any, error) { return f.s.ListRoles(ctx, f.space) }},
		{"GetRoleByID", func(f ordinaryScopeFixture) (any, error) { return f.s.GetRoleByID(ctx, f.custom) }},
		{"AssignMemberRole", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.AssignMemberRole(ctx, f.space, f.emptyActor, f.member, f.owner)
		}},
		{"RevokeMemberRole", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.RevokeMemberRole(ctx, f.space, f.owner, f.custom)
		}},
		{"GetMemberRoles", func(f ordinaryScopeFixture) (any, error) { return f.s.GetMemberRoles(ctx, f.space, f.owner) }},
		{"GetEffectiveMask_owner", func(f ordinaryScopeFixture) (any, error) {
			return f.s.GetEffectiveMask(ctx, f.space, f.owner, &f.chat, &f.voice)
		}},
		{"GetEffectiveMask_empty", func(f ordinaryScopeFixture) (any, error) {
			return f.s.GetEffectiveMask(ctx, f.space, f.emptyActor, nil, nil)
		}},
		{"CanManageRole", func(f ordinaryScopeFixture) (any, error) { return f.s.CanManageRole(ctx, f.space, f.owner, f.custom) }},
		{"CanCreateRole", func(f ordinaryScopeFixture) (any, error) { return f.s.CanCreateRole(ctx, f.space, f.owner, 1) }},
		{"CanEditRole", func(f ordinaryScopeFixture) (any, error) { return f.s.CanEditRole(ctx, f.space, f.owner, f.custom) }},
		{"SetChatOverride", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.SetChatOverride(ctx, f.chat, f.custom, 8, 16)
		}},
		{"SetChatOverrideForMemberRoles", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.SetChatOverrideForMemberRoles(ctx, f.space, f.chat, f.owner, 8, 16)
		}},
		{"SetChatOverrideForMemberRoles_empty", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.SetChatOverrideForMemberRoles(ctx, f.space, f.chat, f.emptyActor, 8, 16)
		}},
		{"SetVoiceRoomOverride", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.SetVoiceRoomOverride(ctx, f.voice, f.custom, 8, 16)
		}},
		{"CreateCustomRole", func(f ordinaryScopeFixture) (any, error) {
			return f.s.CreateCustomRole(ctx, f.space, "forbidden new role", 8, 2, &f.owner)
		}},
		{"UpdateRole", func(f ordinaryScopeFixture) (any, error) {
			name := "forbidden rename"
			return f.s.UpdateRole(ctx, f.custom, &name, nil, nil)
		}},
		{"DeleteRolesCreatedByProfile", func(f ordinaryScopeFixture) (any, error) {
			return f.s.DeleteRolesCreatedByProfile(ctx, f.space, f.owner)
		}},
		{"DeleteRole", func(f ordinaryScopeFixture) (any, error) { return nil, f.s.DeleteRole(ctx, f.custom) }},
		{"ReorderRoles", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.ReorderRoles(ctx, f.space, []uuid.UUID{f.custom, f.member})
		}},
		{"ReorderRoles_empty", func(f ordinaryScopeFixture) (any, error) { return nil, f.s.ReorderRoles(ctx, f.space, nil) }},
		{"ListChatOverrides", func(f ordinaryScopeFixture) (any, error) { return f.s.ListChatOverrides(ctx, f.space, &f.chat) }},
		{"ListVoiceRoomOverrides", func(f ordinaryScopeFixture) (any, error) { return f.s.ListVoiceRoomOverrides(ctx, f.space, &f.voice) }},
		{"RemoveChatOverride", func(f ordinaryScopeFixture) (any, error) { return nil, f.s.RemoveChatOverride(ctx, f.chat, f.custom) }},
		{"RemoveVoiceRoomOverride", func(f ordinaryScopeFixture) (any, error) {
			return nil, f.s.RemoveVoiceRoomOverride(ctx, f.voice, f.custom)
		}},
		{"SetDefaultJoinRole", func(f ordinaryScopeFixture) (any, error) { return nil, f.s.SetDefaultJoinRole(ctx, f.space, f.custom) }},
		{"GetDefaultJoinRole", func(f ordinaryScopeFixture) (any, error) { return f.s.GetDefaultJoinRole(ctx, f.space) }},
		{"RoleIDByNameRow", func(f ordinaryScopeFixture) (any, error) {
			return f.s.RoleIDByNameRow(ctx, f.space, permissions.RoleOwner)
		}},
		{"RoleIDByName", func(f ordinaryScopeFixture) (any, error) {
			id, e := f.s.RoleIDByName(ctx, f.space, permissions.RoleOwner)
			return id != uuid.Nil, e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newOrdinaryScopeFixture(t, ctx, pool)
			prepareOrdinaryScope(t, ctx, pool, f)
			before := ordinaryRowsSnapshot(t, ctx, pool)
			got, err := tc.call(f)
			require.ErrorIs(t, err, ErrSpaceFrozen)
			require.Empty(t, got, "denied operation must not return roles, permissions or mutation counts")
			require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool), "denied operation must not change any ordinary data")
		})
	}

	t.Run("unseeded bootstrap and absent default cannot bypass prepared fence", func(t *testing.T) {
		f := ordinaryScopeFixture{s: &RoleStore{Pool: pool}, space: uuid.New(), owner: uuid.New()}
		prepareOrdinaryScope(t, ctx, pool, f)
		before := ordinaryRowsSnapshot(t, ctx, pool)
		require.ErrorIs(t, f.s.BootstrapSystemRoles(ctx, f.space), ErrSpaceFrozen)
		row, err := f.s.GetDefaultJoinRole(ctx, f.space)
		require.ErrorIs(t, err, ErrSpaceFrozen)
		require.Nil(t, row)
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
	})
}

func TestOrdinaryStoreRetiredAndLookupFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := startOrdinaryScopeDB(t, ctx)
	f := newOrdinaryScopeFixture(t, ctx, pool)
	_, err := pool.Exec(ctx, "INSERT INTO role_space_lifecycle(space_id,retired_at) VALUES ($1,now())", f.space)
	require.NoError(t, err)
	before := ordinaryRowsSnapshot(t, ctx, pool)
	row, err := f.s.GetRoleByID(ctx, f.custom)
	require.ErrorIs(t, err, ErrSpaceRetired)
	require.Nil(t, row)
	require.ErrorIs(t, f.s.BootstrapSpaceRoles(ctx, f.space, uuid.New()), ErrSpaceRetired)
	require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))

	live := newOrdinaryScopeFixture(t, ctx, pool)
	_, err = pool.Exec(ctx, "ALTER TABLE ownership_transfer_v2 RENAME TO missing_ordinary_fence")
	require.NoError(t, err)
	before = ordinaryRowsSnapshot(t, ctx, pool)
	mask, err := live.s.GetEffectiveMask(ctx, live.space, live.owner, nil, nil)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	require.Zero(t, mask)
	row, err = live.s.GetDefaultJoinRole(ctx, live.space)
	require.ErrorIs(t, err, ErrScopeUnavailable)
	require.Nil(t, row)
	require.ErrorIs(t, live.s.AssignMemberRole(ctx, live.space, live.emptyActor, live.member, live.owner), ErrScopeUnavailable)
	require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
}

func TestOrdinaryStoreOwnershipAndForeignRoleInvariants(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := startOrdinaryScopeDB(t, ctx)
	t.Run("bootstrap same owner is idempotent but cannot add another owner", func(t *testing.T) {
		f := newOrdinaryScopeFixture(t, ctx, pool)
		before := ordinaryRowsSnapshot(t, ctx, pool)
		created, err := f.s.BootstrapSpaceRolesWithCreatedSystemRoles(ctx, f.space, f.owner)
		require.NoError(t, err)
		require.Empty(t, created)
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
		created, err = f.s.BootstrapSpaceRolesWithCreatedSystemRoles(ctx, f.space, uuid.New())
		require.Error(t, err)
		require.Empty(t, created)
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
	})
	t.Run("assignment cannot attach another space role", func(t *testing.T) {
		local := newOrdinaryScopeFixture(t, ctx, pool)
		foreign := newOrdinaryScopeFixture(t, ctx, pool)
		before := ordinaryRowsSnapshot(t, ctx, pool)
		require.Error(t, local.s.AssignMemberRole(ctx, local.space, local.emptyActor, foreign.member, local.owner))
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
	})
	t.Run("covered foreign space does not grant local Owner management of its role", func(t *testing.T) {
		local := newOrdinaryScopeFixture(t, ctx, pool)
		foreign := newOrdinaryScopeFixture(t, ctx, pool)
		before := ordinaryRowsSnapshot(t, ctx, pool)
		err := local.s.WithinSpaces(ctx, []uuid.UUID{local.space, foreign.space}, func(scoped *RoleStore) error {
			allowed, _ := scoped.CanManageRole(ctx, local.space, local.owner, foreign.custom)
			require.False(t, allowed, "covering both transaction scopes must not authorize a foreign-space target role")
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
	})
	t.Run("corrupt foreign Owner assignment never grants local authority", func(t *testing.T) {
		local := newOrdinaryScopeFixture(t, ctx, pool)
		foreign := newOrdinaryScopeFixture(t, ctx, pool)
		foreignOwner, err := foreign.s.RoleIDByName(ctx, foreign.space, permissions.RoleOwner)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, "INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES ($1,$2,$3,$4)", local.space, local.emptyActor, foreignOwner, local.owner)
		require.NoError(t, err, "current schema permits this corrupt historical row; reads must fail closed")
		roles, err := local.s.GetMemberRoles(ctx, local.space, local.emptyActor)
		require.Error(t, err)
		require.Empty(t, roles)
		mask, err := local.s.GetEffectiveMask(ctx, local.space, local.emptyActor, nil, nil)
		require.Error(t, err)
		require.Zero(t, mask)
		allowed, err := local.s.CanCreateRole(ctx, local.space, local.emptyActor, 1)
		require.Error(t, err)
		require.False(t, allowed)
	})
	t.Run("ordinary helpers share outer transaction and rollback", func(t *testing.T) {
		f := newOrdinaryScopeFixture(t, ctx, pool)
		before := ordinaryRowsSnapshot(t, ctx, pool)
		sentinel := errors.New("reject entire ordinary operation")
		err := f.s.WithinSpaces(ctx, []uuid.UUID{f.space}, func(scoped *RoleStore) error {
			created, err := scoped.CreateCustomRole(ctx, f.space, "uncommitted role", 1, 3, &f.owner)
			if err != nil {
				return err
			}
			loaded, err := scoped.GetRoleByID(ctx, created.ID)
			if err != nil {
				return err
			}
			require.NotNil(t, loaded)
			require.Equal(t, created.ID, loaded.ID)
			rows, err := scoped.ListRoles(ctx, f.space)
			if err != nil {
				return err
			}
			found := false
			for _, row := range rows {
				if row.ID == created.ID {
					found = true
				}
			}
			require.True(t, found, "scoped reads must see the pending write")
			var outside int
			require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM roles WHERE id=$1", created.ID).Scan(&outside))
			require.Zero(t, outside, "ordinary helper must not independently commit through Pool")
			if err := scoped.SetDefaultJoinRole(ctx, f.space, f.custom); err != nil {
				return err
			}
			if err := scoped.ReorderRoles(ctx, f.space, []uuid.UUID{f.custom, f.member}); err != nil {
				return err
			}
			if err := scoped.SetChatOverrideForMemberRoles(ctx, f.space, uuid.New(), f.owner, 8, 16); err != nil {
				return err
			}
			return sentinel
		})
		require.ErrorIs(t, err, sentinel)
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
	})
	t.Run("reorder rejects foreign role without a partial local update", func(t *testing.T) {
		local := newOrdinaryScopeFixture(t, ctx, pool)
		foreign := newOrdinaryScopeFixture(t, ctx, pool)
		before := ordinaryRowsSnapshot(t, ctx, pool)
		require.Error(t, local.s.ReorderRoles(ctx, local.space, []uuid.UUID{local.member, foreign.custom}))
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool))
	})
	t.Run("member override batch rolls back first write when second fails", func(t *testing.T) {
		f := newOrdinaryScopeFixture(t, ctx, pool)
		first, err := f.s.CreateCustomRole(ctx, f.space, "first batch role", 1, 20, &f.owner)
		require.NoError(t, err)
		require.NoError(t, f.s.AssignMemberRole(ctx, f.space, f.emptyActor, first.ID, f.owner))
		require.NoError(t, f.s.AssignMemberRole(ctx, f.space, f.emptyActor, f.custom, f.owner))
		roles, err := f.s.GetMemberRoles(ctx, f.space, f.emptyActor)
		require.NoError(t, err)
		require.Len(t, roles, 2)
		require.Equal(t, first.ID, roles[0].ID)
		require.Equal(t, f.custom, roles[1].ID)
		// The trigger argument is a generated UUID, compared as UUID in the
		// function. The second ordered role must reach PostgreSQL and fail.
		_, err = pool.Exec(ctx, `CREATE FUNCTION reject_second_scope_override() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.role_id = TG_ARGV[0]::uuid THEN RAISE EXCEPTION 'second scope override rejected' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER reject_second_scope_override BEFORE INSERT OR UPDATE ON chat_overrides
FOR EACH ROW EXECUTE FUNCTION reject_second_scope_override('`+f.custom.String()+`');`)
		require.NoError(t, err)
		defer func() {
			_, cleanupErr := pool.Exec(ctx, "DROP TRIGGER reject_second_scope_override ON chat_overrides; DROP FUNCTION reject_second_scope_override()")
			require.NoError(t, cleanupErr)
		}()
		before := ordinaryRowsSnapshot(t, ctx, pool)
		err = f.s.SetChatOverrideForMemberRoles(ctx, f.space, uuid.New(), f.emptyActor, 8, 16)
		require.ErrorContains(t, err, "second scope override rejected")
		require.Equal(t, before, ordinaryRowsSnapshot(t, ctx, pool), "first override must roll back with the failing second override")
	})
}
