package store

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func authorityRevisionSpace(t *testing.T) (context.Context, *SpaceStore) {
	return authorityRevisionSpaceThrough(t, 24)
}

func authorityRevisionSpaceThrough(t *testing.T, lastVersion int) (context.Context, *SpaceStore) {
	t.Helper()
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/space_db")
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		require.NoError(t, err)
		if version <= lastVersion {
			_, err = pool.Exec(ctx, r22SpaceMigrationSQL(t, file.Name()))
			require.NoError(t, err)
		}
	}
	st := &SpaceStore{Pool: pool}
	return ctx, st
}

func TestSpaceAuthorityRevisionCoversBansLifecycleAndBothUpdateScopes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL authority revisions")
	}
	ctx, st := authorityRevisionSpace(t)
	owner, member := uuid.New(), uuid.New()
	room, account := uuid.New(), uuid.New()
	first, err := st.CreateSpace(ctx, owner, "revision first", "", "private")
	require.NoError(t, err)
	second, err := st.CreateSpace(ctx, owner, "revision second", "", "private")
	require.NoError(t, err)
	mutations := []struct {
		name, sql string
		args      []any
		both      bool
	}{
		{"account ban", `INSERT INTO space_bans(space_id,account_id,banned_by_profile_id) VALUES($1,$2,$3)`, []any{first.ID, account, owner}, false},
		{"account unban", `DELETE FROM space_bans WHERE space_id=$1 AND account_id=$2`, []any{first.ID, account}, false},
		{"member timeout", `INSERT INTO space_member_timeouts(space_id,profile_id,timed_out_until,timed_out_by_profile_id) VALUES($1,$2,now()+interval '1 hour',$3)`, []any{first.ID, member, owner}, false},
		{"lifecycle freeze", `INSERT INTO space_lifecycle_aggregates(space_id,deletion_operation_id,phase,generation) VALUES($1,$2,'FREEZE_PENDING',1)`, []any{first.ID, uuid.New()}, false},
		{"lifecycle decision", `UPDATE space_lifecycle_aggregates SET phase='SCHEDULED' WHERE space_id=$1`, []any{first.ID}, false},
		{"owner change", `UPDATE spaces SET owner_profile_id=$2 WHERE id=$1`, []any{first.ID, member}, false},
		{"ownership reserve", `INSERT INTO ownership_journal(operation_id,protocol_version,space_id,account_id,actor_profile_id,new_owner_profile_id,session_epoch,proof_digest,binding_bytes,binding_hash,audit_id,event_id) VALUES(gen_random_uuid(),2,$1,gen_random_uuid(),$2,$3,1,repeat('0',64),decode('01','hex'),decode(repeat('00',32),'hex'),gen_random_uuid(),gen_random_uuid())`, []any{first.ID, owner, member}, false},
		{"ownership abort", `UPDATE ownership_journal SET state='aborted',role_terminal_receipt_bytes=decode('01','hex'),role_terminal_receipt_hash=decode(repeat('00',32),'hex') WHERE space_id=$1`, []any{first.ID}, false},
		{"room insert", `INSERT INTO voice_rooms(id,space_id,name) VALUES($1,$2,'authority room')`, []any{room, first.ID}, false},
		{"room scope move", `UPDATE voice_rooms SET space_id=$2 WHERE id=$1`, []any{room, second.ID}, true},
		{"member move setup", `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, []any{first.ID, member}, false},
		{"member scope move", `UPDATE space_members SET space_id=$2 WHERE space_id=$1 AND profile_id=$3`, []any{first.ID, second.ID, member}, true},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			before, control := r22SpaceEpoch(t, ctx, st, first.ID), r22SpaceEpoch(t, ctx, st, second.ID)
			_, err := st.Pool.Exec(ctx, mutation.sql, mutation.args...)
			require.NoError(t, err)
			after := r22SpaceEpoch(t, ctx, st, first.ID)
			require.Greater(t, after, before, "every authority change must invalidate its old scope")
			requireR22SpaceSnapshot(t, r22SpaceOutboxRow(t, ctx, st, first.ID, after), first.ID, after)
			if mutation.both {
				require.Greater(t, r22SpaceEpoch(t, ctx, st, second.ID), control)
			} else {
				require.Equal(t, control, r22SpaceEpoch(t, ctx, st, second.ID))
			}
		})
	}
}

func TestSpaceAuthorityRevisionCannotBeRewoundRemovedOrTruncated(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL durable revision floors")
	}
	ctx, st := authorityRevisionSpace(t)
	space, err := st.CreateSpace(ctx, uuid.New(), "revision guard", "", "private")
	require.NoError(t, err)
	before := r22SpaceEpoch(t, ctx, st, space.ID)
	for _, sql := range []string{
		`UPDATE space_voice_access_epochs SET access_epoch=1 WHERE space_id=$1`,
		`DELETE FROM space_voice_access_epochs WHERE space_id=$1`,
		`UPDATE space_voice_access_epochs SET space_id=gen_random_uuid() WHERE space_id=$1`,
	} {
		_, err := st.Pool.Exec(ctx, sql, space.ID)
		require.Error(t, err)
		require.Equal(t, before, r22SpaceEpoch(t, ctx, st, space.ID))
	}
	_, err = st.Pool.Exec(ctx, `TRUNCATE space_voice_access_epochs`)
	require.Error(t, err)
	require.Equal(t, before, r22SpaceEpoch(t, ctx, st, space.ID))
	_, err = st.Pool.Exec(ctx, `TRUNCATE space_bans`)
	require.Error(t, err, "authority TRUNCATE must not bypass row invalidation")
}

func TestSpaceAuthorityRevisionUpgradeInvalidatesSavedScopesAndCannotDowngrade(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL authority upgrade")
	}
	ctx, st := authorityRevisionSpaceThrough(t, 23)
	space, err := st.CreateSpace(ctx, uuid.New(), "saved authority", "", "private")
	require.NoError(t, err)
	before := r22SpaceEpoch(t, ctx, st, space.ID)
	out := r22SpaceOutboxRow(t, ctx, st, space.ID, before)
	_, err = st.Pool.Exec(ctx, r22SpaceMigrationSQL(t, "000024_authority_revision.up.sql"))
	require.NoError(t, err)
	require.Equal(t, before+1, r22SpaceEpoch(t, ctx, st, space.ID))
	require.Equal(t, out, r22SpaceOutboxRow(t, ctx, st, space.ID, before), "upgrade preserves immutable previous invalidations")
	_, err = st.Pool.Exec(ctx, r22SpaceMigrationSQL(t, "000024_authority_revision.down.sql"))
	require.Error(t, err)
	require.Equal(t, before+1, r22SpaceEpoch(t, ctx, st, space.ID))
}

func TestSpaceAuthorityRevisionUpgradeFailurePreservesPreviousSchemaAndFloors(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL atomic authority migration")
	}
	for name, mutation := range map[string]string{
		"filtered source":         `ALTER TABLE space_bans ENABLE ROW LEVEL SECURITY`,
		"legacy regressed floor":  `UPDATE space_voice_access_epochs SET access_epoch=1`,
		"late activation failure": `CREATE FUNCTION reject_authority_activation() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fixture denies activation' USING ERRCODE='55000'; END $$; CREATE TRIGGER reject_authority_activation BEFORE INSERT ON space_voice_access_outbox FOR EACH ROW EXECUTE FUNCTION reject_authority_activation()`,
	} {
		t.Run(name, func(t *testing.T) {
			ctx, st := authorityRevisionSpaceThrough(t, 23)
			space, err := st.CreateSpace(ctx, uuid.New(), "saved failed upgrade", "", "private")
			require.NoError(t, err)
			_, err = st.Pool.Exec(ctx, mutation)
			require.NoError(t, err)
			before := r22SpaceEpoch(t, ctx, st, space.ID)
			conn, err := st.Pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, r22SpaceMigrationSQL(t, "000024_authority_revision.up.sql"))
			require.Error(t, err)
			_, err = conn.Exec(ctx, `ROLLBACK`)
			require.NoError(t, err)
			conn.Release()
			require.Equal(t, before, r22SpaceEpoch(t, ctx, st, space.ID))
			var newFunction, originalTrigger bool
			require.NoError(t, st.Pool.QueryRow(ctx, `SELECT to_regprocedure('space_authority_revision_changed()') IS NOT NULL,EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='space_members'::regclass AND tgname='space_voice_access_member_change')`).Scan(&newFunction, &originalTrigger))
			require.False(t, newFunction, "failed activation must roll back newly installed coverage")
			require.True(t, originalTrigger, "failure must retain the prior runtime schema")
		})
	}
}
