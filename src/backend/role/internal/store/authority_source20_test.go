package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
)

func roleSourceDDL(t *testing.T, ctx context.Context, st *RoleStore) string {
	t.Helper()
	functions := make([]string, 0, len(roleAuthorityCatalog.Functions))
	for _, f := range roleAuthorityCatalog.Functions {
		functions = append(functions, f.Name)
	}
	var ddl string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT string_agg(item,E'\n' ORDER BY item) FROM (
 SELECT 'R|'||c.relname||'|'||c.relkind::text||'|'||c.relpersistence::text||'|'||c.relrowsecurity||'|'||c.relforcerowsecurity AS item FROM pg_class c WHERE c.relnamespace='public'::regnamespace AND c.relname=ANY($1)
 UNION ALL SELECT 'A|'||c.relname||'|'||a.attnum||'|'||a.attname||'|'||format_type(a.atttypid,a.atttypmod)||'|'||a.attnotnull||'|'||COALESCE(pg_get_expr(d.adbin,d.adrelid),'') FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum WHERE c.relnamespace='public'::regnamespace AND c.relname=ANY($1) AND a.attnum>0 AND NOT a.attisdropped
 UNION ALL SELECT 'C|'||c.relname||'|'||con.conname||'|'||pg_get_constraintdef(con.oid) FROM pg_constraint con JOIN pg_class c ON c.oid=con.conrelid WHERE c.relnamespace='public'::regnamespace AND c.relname=ANY($1)
 UNION ALL SELECT 'I|'||c.relname||'|'||pg_get_indexdef(i.indexrelid) FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid WHERE c.relnamespace='public'::regnamespace AND c.relname=ANY($1)
 UNION ALL SELECT 'T|'||c.relname||'|'||pg_get_triggerdef(t.oid)||'|'||t.tgenabled::text FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid WHERE c.relnamespace='public'::regnamespace AND c.relname=ANY($1) AND NOT t.tgisinternal
 UNION ALL SELECT 'F|'||pg_get_functiondef(p.oid) FROM pg_proc p WHERE p.pronamespace='public'::regnamespace AND p.proname=ANY($2)) catalog`, roleAuthorityCatalog.Tables, functions).Scan(&ddl))
	return ddl
}

func TestRoleSource20ActualUpgradePreservesProjection(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real owner migrator")
	}
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	fixture.Run(true, "goto", "16")
	st := &RoleStore{Pool: fixture.Pool}
	space, chat, room := uuid.New(), uuid.New(), uuid.New()
	ids, profiles := make([]uuid.UUID, 5), make([]uuid.UUID, 5)
	names := []string{"Owner", "Admin", "Moderator", "Member", "custom"}
	for i, name := range names {
		ids[i], profiles[i] = uuid.New(), uuid.New()
		_, err := st.Pool.Exec(ctx, `INSERT INTO roles(id,space_id,name,permissions,is_system,is_default_join) VALUES($1,$2,$3,1,$4,$5)`, ids[i], space, name, i < 4, name == "Member")
		require.NoError(t, err)
		_, err = st.Pool.Exec(ctx, `INSERT INTO member_roles(space_id,profile_id,role_id,assigned_by) VALUES($1,$2,$3,$2)`, space, profiles[i], ids[i])
		require.NoError(t, err)
	}
	control := uuid.New()
	_, err := st.Pool.Exec(ctx, `INSERT INTO roles(id,space_id,name,permissions,is_system) VALUES($1,$2,'Admin',1,false)`, control, uuid.New())
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO chat_overrides(chat_id,role_id,allow,deny) VALUES($1,$2,8,1);`, chat, ids[4])
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO voice_room_overrides(voice_room_id,role_id,allow,deny) VALUES($1,$2,16,1)`, room, ids[4])
	require.NoError(t, err)
	scope := &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()}
	before, err := st.ReadAuthoritySnapshot(ctx, scope)
	require.NoError(t, err)
	beforeState, err := authoritysource.DecodeRoleState(before.CanonicalState)
	require.NoError(t, err)
	ddl := roleSourceDDL(t, ctx, st)
	fixture.Run(true, "up")
	var version int64
	var dirty bool
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.EqualValues(t, 20, version)
	require.False(t, dirty)
	require.Equal(t, ddl, roleSourceDDL(t, ctx, st))
	tx, err := authoritysource.BeginRead(ctx, st.Pool, roleAuthorityCatalog20)
	require.NoError(t, err)
	defer authoritysource.RollbackRead(ctx, tx)
	after, err := readRoleAuthoritySnapshot(ctx, tx, scope)
	require.NoError(t, err)
	state, err := authoritysource.DecodeRoleState(after.CanonicalState)
	require.NoError(t, err)
	require.Equal(t, beforeState.RoleRevision+8, state.RoleRevision, "eight intentional changed-role backfills invalidate the cut")
	require.Equal(t, beforeState.SDKRevision, state.SDKRevision)
	want := []uint64{1 | 1<<42 | 1<<43 | 1<<44, 1 | 4 | 1<<42 | 1<<43 | 1<<44, 1 | 1<<42 | 1<<44, 1 | 1<<44, 1}
	for i, id := range ids {
		var mask uint64
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT permissions FROM roles WHERE id=$1`, id).Scan(&mask))
		require.Equal(t, want[i], mask, names[i])
	}
	var unchanged uint64
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT permissions FROM roles WHERE id=$1`, control).Scan(&unchanged))
	require.EqualValues(t, 1, unchanged, "custom Admin is not a builtin")
	for _, profile := range append(profiles, uuid.New()) {
		for _, voice := range []bool{false, true} {
			resource := chat
			var canonical uint64
			if voice {
				resource = room
				canonical, err = st.GetEffectiveMask(ctx, space, profile, nil, &room)
			} else {
				canonical, err = st.GetEffectiveMask(ctx, space, profile, &chat, nil)
			}
			require.NoError(t, err)
			require.Equal(t, canonical, state.EffectiveMask(profile.String(), resource.String(), voice))
		}
	}
	// A missing/disabled new invalidation input must never qualify catalog20.
	checkRole20Triggers(t, ctx, st, func() error {
		tx, err := authoritysource.BeginRead(ctx, st.Pool, roleAuthorityCatalog20)
		if tx != nil {
			authoritysource.RollbackRead(ctx, tx)
		}
		return err
	})
}

func checkRole20Triggers(t *testing.T, ctx context.Context, st *RoleStore, check func() error) {
	t.Helper()
	for _, trigger := range roleAuthorityCatalog20.Triggers[len(roleAuthorityCatalog.Triggers):] {
		quoted := pgx.Identifier{trigger.Table}.Sanitize()
		name := pgx.Identifier{trigger.Name}.Sanitize()
		var original string
		require.NoError(t, st.Pool.QueryRow(ctx, `SELECT pg_get_triggerdef(oid) FROM pg_trigger WHERE tgrelid=to_regclass($1) AND tgname=$2`, trigger.Table, trigger.Name).Scan(&original))
		for _, mutation := range []string{"ALTER TABLE " + quoted + " DISABLE TRIGGER " + name, "DROP TRIGGER " + name + " ON " + quoted} {
			_, err := st.Pool.Exec(ctx, mutation)
			require.NoError(t, err)
			require.Error(t, check())
			if mutation[0] == 'A' {
				_, err = st.Pool.Exec(ctx, "ALTER TABLE "+quoted+" ENABLE TRIGGER "+name)
			} else {
				_, err = st.Pool.Exec(ctx, original)
			}
			require.NoError(t, err)
		}
	}
}

func TestRoleSource20ServingSelectorAndGuards(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real owner migrator")
	}
	ctx := context.Background()
	fixture := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	fixture.Run(true, "up")
	st := &RoleStore{Pool: fixture.Pool}
	check := func() error { return st.CheckAuthoritySourceSchema(ctx) }
	require.NoError(t, check())
	for _, version := range []int64{18, 19, 21, 999} {
		_, err := st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=$1`, version)
		require.NoError(t, err)
		require.Error(t, check())
	}
	_, err := st.Pool.Exec(ctx, `UPDATE schema_migrations SET version=20,dirty=true`)
	require.NoError(t, err)
	require.Error(t, check())
	_, err = st.Pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
	require.NoError(t, err)
	require.NoError(t, check())
	checkRole20Triggers(t, ctx, st, check)
	require.NoError(t, check())
}
