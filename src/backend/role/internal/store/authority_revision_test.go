package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func authorityRevisionRole(t *testing.T) (context.Context, *RoleStore) {
	ctx, st := authorityRevisionRoleBeforeUpgrade(t)
	_, err := st.Pool.Exec(ctx, role14SQL(t, "000015_authority_revision.up.sql"))
	require.NoError(t, err)
	return ctx, st
}

func authorityRevisionRoleBeforeUpgrade(t *testing.T) (context.Context, *RoleStore) {
	t.Helper()
	ctx, pool := role14Base(t)
	_, err := pool.Exec(ctx, role14SQL(t, "000013_game_session_grants.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.up.sql"))
	require.NoError(t, err)
	return ctx, &RoleStore{Pool: pool}
}

func TestRoleAuthorityRevisionCoversChatFencesAndBothUpdateScopes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL authority revisions")
	}
	ctx, st := authorityRevisionRole(t)
	pool := st.Pool
	first, second, role, control := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	room, chat, profile := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO roles(id,space_id,name) VALUES($1,$2,'revision first'),($3,$4,'revision control')`, role, first, control, second)
	require.NoError(t, err)
	mutations := []struct {
		name, sql string
		args      []any
		both      bool
	}{
		{"chat override", `INSERT INTO chat_overrides(chat_id,role_id,deny) VALUES($1,$2,1)`, []any{chat, role}, false},
		{"chat override update", `UPDATE chat_overrides SET deny=2 WHERE chat_id=$1`, []any{chat}, false},
		{"voice override", `INSERT INTO voice_room_overrides(voice_room_id,role_id,deny) VALUES($1,$2,1)`, []any{room, role}, false},
		{"override scope move", `UPDATE voice_room_overrides SET role_id=$2 WHERE voice_room_id=$1`, []any{room, control}, true},
		{"member assignment", `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$4)`, []any{first, profile, role, uuid.New()}, false},
		{"assignment scope move", `UPDATE member_roles SET space_id=$2,role_id=$3 WHERE space_id=$1`, []any{first, second, control}, true},
		{"lifecycle fence", `INSERT INTO role_space_deletion_fences VALUES($1,$2,1,'FROZEN',$3,$4,0)`, []any{first, uuid.New(), uuid.New(), make([]byte, 32)}, false},
		{"lifecycle restore", `UPDATE role_space_deletion_fences SET generation=2,state='LIVE' WHERE space_id=$1`, []any{first}, false},
		{"role scope move", `UPDATE roles SET space_id=$2 WHERE id=$1`, []any{role, second}, true},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			before, control := r22RoleEpoch(t, ctx, st, first), r22RoleEpoch(t, ctx, st, second)
			_, err := pool.Exec(ctx, mutation.sql, mutation.args...)
			require.NoError(t, err)
			after := r22RoleEpoch(t, ctx, st, first)
			require.Greater(t, after, before, "every authority change must invalidate its old scope")
			requireR22RoleSnapshot(t, r22RoleOutboxRow(t, ctx, st, first, after), first, after)
			if mutation.both {
				require.Greater(t, r22RoleEpoch(t, ctx, st, second), control)
			} else {
				require.Equal(t, control, r22RoleEpoch(t, ctx, st, second))
			}
		})
	}
}

func TestRoleAuthorityRevisionCannotBeRewoundRemovedOrTruncated(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL durable revision floors")
	}
	ctx, st := authorityRevisionRole(t)
	space := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'revision guard'),($1,'revision guard second')`, space)
	require.NoError(t, err)
	before := r22RoleEpoch(t, ctx, st, space)
	for _, sql := range []string{
		`UPDATE role_voice_policy_epochs SET policy_epoch=1 WHERE space_id=$1`,
		`DELETE FROM role_voice_policy_epochs WHERE space_id=$1`,
		`UPDATE role_voice_policy_epochs SET space_id=gen_random_uuid() WHERE space_id=$1`,
	} {
		_, err := st.Pool.Exec(ctx, sql, space)
		require.Error(t, err)
		require.Equal(t, before, r22RoleEpoch(t, ctx, st, space))
	}
	_, err = st.Pool.Exec(ctx, `TRUNCATE role_voice_policy_epochs`)
	require.Error(t, err)
	require.Equal(t, before, r22RoleEpoch(t, ctx, st, space))
	_, err = st.Pool.Exec(ctx, `TRUNCATE role_space_deletion_fences`)
	require.Error(t, err, "authority TRUNCATE must not bypass row invalidation")
}

func TestRoleAuthorityRevisionUpgradeInvalidatesSavedScopesAndCannotDowngrade(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL authority upgrade")
	}
	ctx, st := authorityRevisionRoleBeforeUpgrade(t)
	space := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'saved authority')`, space)
	require.NoError(t, err)
	role14FenceEvidence(t, ctx, st.Pool)
	before := r22RoleEpoch(t, ctx, st, space)
	out := r22RoleOutboxRow(t, ctx, st, space, before)
	rows := role14Rows(t, ctx, st.Pool, "role_space_deletion_fence_receipts")
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000015_authority_revision.up.sql"))
	require.NoError(t, err)
	require.Equal(t, before+1, r22RoleEpoch(t, ctx, st, space))
	require.Equal(t, out, r22RoleOutboxRow(t, ctx, st, space, before))
	require.Equal(t, rows, role14Rows(t, ctx, st.Pool, "role_space_deletion_fence_receipts"))
	_, err = st.Pool.Exec(ctx, role14SQL(t, "000015_authority_revision.down.sql"))
	require.Error(t, err)
	require.Equal(t, before+1, r22RoleEpoch(t, ctx, st, space))
}

func TestRoleAuthorityRevisionUpgradeFailurePreservesPreviousSchemaAndFloors(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL atomic authority migration")
	}
	for name, mutation := range map[string]string{
		"filtered source":         `ALTER TABLE chat_overrides ENABLE ROW LEVEL SECURITY`,
		"legacy regressed floor":  `UPDATE role_voice_policy_epochs SET policy_epoch=1`,
		"late activation failure": `CREATE FUNCTION reject_authority_activation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture denies activation' USING ERRCODE='55000'; END $$; CREATE TRIGGER reject_authority_activation BEFORE INSERT ON role_voice_policy_outbox FOR EACH ROW EXECUTE FUNCTION reject_authority_activation()`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, st := authorityRevisionRoleBeforeUpgrade(t)
			space := uuid.New()
			_, err := st.Pool.Exec(ctx, `INSERT INTO roles(space_id,name) VALUES($1,'saved first'),($1,'saved second')`, space)
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, mutation)
			require.NoError(t, err)
			before := r22RoleEpoch(t, ctx, st, space)
			conn, err := st.Pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, role14SQL(t, "000015_authority_revision.up.sql"))
			require.Error(t, err)
			_, err = conn.Exec(ctx, `ROLLBACK`)
			require.NoError(t, err)
			conn.Release()
			require.Equal(t, before, r22RoleEpoch(t, ctx, st, space))
			var newFunction, originalTrigger bool
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regprocedure('role_authority_revision_changed()') IS NOT NULL,EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='roles'::regclass AND tgname='role_voice_policy_role_change')`).Scan(&newFunction, &originalTrigger))
			require.False(t, newFunction)
			require.True(t, originalTrigger)
		})
	}
}
