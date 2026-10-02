package store

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func role14SQL(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/role_db", name))
	require.NoError(t, err)
	return string(raw)
}

func role14Base(t *testing.T) (context.Context, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool := StartRoleDBForStoreTest(t, ctx)
	directory := filepath.Join(repoRoot(t), "src/backend/migrations/role_db")
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") {
			continue
		}
		version, err := strconv.Atoi(strings.SplitN(file.Name(), "_", 2)[0])
		require.NoError(t, err)
		if version <= 12 {
			_, err = pool.Exec(ctx, role14SQL(t, file.Name()))
			require.NoError(t, err)
		}
	}
	_, err = pool.Exec(ctx, `CREATE TABLE public.schema_migrations(version BIGINT PRIMARY KEY, dirty BOOLEAN NOT NULL); INSERT INTO public.schema_migrations VALUES(13,false)`)
	require.NoError(t, err)
	return ctx, pool
}

func role14LegacyFence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/role/internal/store/testdata/legacy-role13-fence.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(raw))
	require.NoError(t, err)
}

func role14FenceEvidence(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	space, operation, manifest := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO role_space_deletion_fences VALUES($1,$2,1,'FROZEN',$3,$4,0)`, space, operation, manifest, make([]byte, 32))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO role_space_deletion_fence_receipts(space_id,deletion_operation_id,generation,state,manifest_id,manifest_sha256,manifest_item_count,receipt_id,applied_at,request_sha256,request_bytes,receipt_bytes) VALUES($1,$2,1,'FROZEN',$3,$4,0,$5,clock_timestamp(),$4,$6,$7)`, space, operation, manifest, make([]byte, 32), uuid.New(), []byte{1, 2, 3}, []byte{4, 5, 6})
	require.NoError(t, err)
}

func role14Rows(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) string {
	t.Helper()
	var rows string
	require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text),'[]'::jsonb)::text FROM public.`+table+` t`).Scan(&rows))
	return rows
}

func TestRoleMigration14AdoptsExactHistoricalSchemasWithoutChangingEvidence(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL migration catalog")
	}
	for _, shape := range []string{"canonical grants", "legacy fences", "combined", "migrator dirty14"} {
		t.Run(shape, func(t *testing.T) {
			ctx, pool := role14Base(t)
			before := map[string]string{}
			legacyFences := shape == "legacy fences" || shape == "combined"
			if shape != "legacy fences" {
				_, err := pool.Exec(ctx, role14SQL(t, "000013_game_session_grants.up.sql"))
				require.NoError(t, err)
				_, err = pool.Exec(ctx, `INSERT INTO game_session_grant_sessions(application_id,environment_id,session_id,roster_revision,profile_set_sha256,status) VALUES($1,$2,$3,0,$4,'revoked')`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32))
				require.NoError(t, err)
				before["game_session_grant_sessions"] = role14Rows(t, ctx, pool, "game_session_grant_sessions")
			}
			if legacyFences {
				role14LegacyFence(t, ctx, pool)
				role14FenceEvidence(t, ctx, pool)
				for _, table := range []string{"role_space_deletion_fences", "role_space_deletion_fence_receipts"} {
					before[table] = role14Rows(t, ctx, pool, table)
				}
			}
			if shape == "migrator dirty14" {
				_, err := pool.Exec(ctx, `UPDATE schema_migrations SET version=14,dirty=true`)
				require.NoError(t, err)
			}
			_, err := pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.up.sql"))
			require.NoError(t, err)
			for table, rows := range before {
				require.Equal(t, rows, role14Rows(t, ctx, pool, table), "upgrade must retain exact saved rows and bytes")
			}
			for _, table := range []string{"game_session_grant_sessions", "game_session_grants", "game_session_grant_operations", "role_space_deletion_fences", "role_space_deletion_fence_receipts"} {
				var present bool
				require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.'||$1) IS NOT NULL`, table).Scan(&present))
				require.True(t, present)
			}
			if legacyFences {
				_, err = pool.Exec(ctx, `UPDATE role_space_deletion_fence_receipts SET generation=2`)
				require.Error(t, err, "adoption must preserve immutable receipt enforcement")
				_, err = pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.down.sql"))
				require.Error(t, err, "saved fence evidence prevents rollback")
				for table, rows := range before {
					require.Equal(t, rows, role14Rows(t, ctx, pool, table))
				}
			} else {
				_, err = pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.down.sql"))
				require.NoError(t, err)
				require.Equal(t, before["game_session_grant_sessions"], role14Rows(t, ctx, pool, "game_session_grant_sessions"), "down retains canonical v13 game grants")
			}
		})
	}
}

func TestRoleMigration14RejectsPartialAlteredAndInvalidVersionState(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL migration catalog")
	}
	cases := map[string]string{
		"partial fence":     `DROP TABLE role_space_deletion_fence_receipts`,
		"wrong column":      `ALTER TABLE role_space_deletion_fences ALTER COLUMN generation TYPE numeric`,
		"wrong constraint":  `ALTER TABLE role_space_deletion_fences DROP CONSTRAINT role_space_deletion_fences_generation_check`,
		"disabled trigger":  `ALTER TABLE role_space_deletion_fence_receipts DISABLE TRIGGER role_deletion_fence_receipt_immutable`,
		"missing trigger":   `DROP TRIGGER role_deletion_fence_receipt_immutable ON role_space_deletion_fence_receipts`,
		"altered function":  `ALTER FUNCTION role_deletion_fence_receipt_immutable() SECURITY DEFINER`,
		"row security":      `ALTER TABLE role_space_deletion_fences ENABLE ROW LEVEL SECURITY`,
		"forced security":   `ALTER TABLE role_space_deletion_fence_receipts FORCE ROW LEVEL SECURITY`,
		"inactive policy":   `CREATE POLICY hidden_evidence ON role_space_deletion_fences USING(false)`,
		"rewrite rule":      `CREATE RULE suppress_fence_insert AS ON INSERT TO role_space_deletion_fences DO INSTEAD NOTHING`,
		"unlogged evidence": `ALTER TABLE role_space_deletion_fence_receipts SET UNLOGGED`,
		"inherited fence":   `CREATE TABLE inherited_fences () INHERITS(role_space_deletion_fences)`,
		"dirty source":      `UPDATE schema_migrations SET dirty=true`,
		"old source":        `UPDATE schema_migrations SET version=12`,
		"unrecorded source": `DROP TABLE schema_migrations`,
		"multiple markers":  `INSERT INTO schema_migrations VALUES(14,true)`,
		"completed14":       `UPDATE schema_migrations SET version=14`,
		"empty source":      `DROP TABLE role_space_deletion_fence_receipts,role_space_deletion_fences; DROP FUNCTION role_deletion_fence_receipt_immutable()`,
		"partial grants":    `DROP TABLE game_session_grant_operations`,
		"missing index":     `DROP INDEX game_session_grants_voice_lookup_idx`,
		"wrong FK":          `DO $$ DECLARE fk TEXT; BEGIN SELECT conname INTO fk FROM pg_constraint WHERE conrelid='game_session_grants'::regclass AND contype='f'; EXECUTE format('ALTER TABLE game_session_grants DROP CONSTRAINT %I',fk); ALTER TABLE game_session_grants ADD FOREIGN KEY(application_id,environment_id,session_id) REFERENCES game_session_grant_sessions(application_id,environment_id,session_id) ON DELETE CASCADE; END $$`,
	}
	for name, mutation := range cases {
		t.Run(name, func(t *testing.T) {
			ctx, pool := role14Base(t)
			role14LegacyFence(t, ctx, pool)
			if name == "partial grants" || name == "missing index" || name == "wrong FK" {
				_, err := pool.Exec(ctx, role14SQL(t, "000013_game_session_grants.up.sql"))
				require.NoError(t, err)
			}
			_, err := pool.Exec(ctx, mutation)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.up.sql"))
			require.Error(t, err, "unknown or invalid source state must not be adopted")
			var created bool
			require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.game_session_grant_sessions') IS NOT NULL`).Scan(&created))
			if name == "partial grants" || name == "missing index" || name == "wrong FK" {
				require.True(t, created, "failed upgrade must preserve existing grant schema")
			} else {
				require.False(t, created, "failed upgrade must not leave partial grant schema")
			}
		})
	}
}

func TestRoleMigration14RollbackWaitsForConcurrentEvidenceThenRefuses(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL migration barrier")
	}
	ctx, pool := role14Base(t)
	_, err := pool.Exec(ctx, role14SQL(t, "000013_game_session_grants.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.up.sql"))
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
	_, err = tx.Exec(ctx, `INSERT INTO role_space_deletion_fences VALUES($1,$2,1,'FROZEN',$3,$4,0)`, uuid.New(), uuid.New(), uuid.New(), make([]byte, 32))
	require.NoError(t, err)
	const application = "role14-down-evidence-barrier"
	second := r22SecondRolePool(t, ctx, pool, application)
	ddl := role14SQL(t, "000014_space_deletion_fence.down.sql")
	result := make(chan error, 1)
	go func() { _, err := second.Exec(ctx, ddl); result <- err }()
	r22WaitForRoleLock(t, ctx, pool, application)
	require.NoError(t, tx.Commit(ctx))
	require.Error(t, <-result, "rollback must recheck evidence after waiting")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM role_space_deletion_fences`).Scan(&count))
	require.Equal(t, 1, count)
	var triggerPresent bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='role_space_deletion_fence_receipts'::regclass AND tgname='role_deletion_fence_receipt_immutable' AND tgenabled='O')`).Scan(&triggerPresent))
	require.True(t, triggerPresent)
}

func TestRoleMigration14RollbackCannotLoseEvidenceHiddenByForcedRowSecurity(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL row-security rollback barrier")
	}
	ctx, pool := role14Base(t)
	_, err := pool.Exec(ctx, role14SQL(t, "000013_game_session_grants.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.up.sql"))
	require.NoError(t, err)
	role14FenceEvidence(t, ctx, pool)
	before := role14Rows(t, ctx, pool, "role_space_deletion_fence_receipts")
	_, err = pool.Exec(ctx, `CREATE ROLE role14_migration_fixture NOSUPERUSER NOBYPASSRLS;
GRANT role14_migration_fixture TO CURRENT_USER;
ALTER TABLE role_space_deletion_fences OWNER TO role14_migration_fixture;
ALTER TABLE role_space_deletion_fence_receipts OWNER TO role14_migration_fixture;
ALTER FUNCTION role_deletion_fence_receipt_immutable() OWNER TO role14_migration_fixture;
ALTER TABLE role_space_deletion_fences ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_space_deletion_fences FORCE ROW LEVEL SECURITY;
ALTER TABLE role_space_deletion_fence_receipts ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_space_deletion_fence_receipts FORCE ROW LEVEL SECURITY;`)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = tx.Exec(ctx, `SET LOCAL ROLE role14_migration_fixture; SET LOCAL row_security=on`)
	require.NoError(t, err)
	var visible int
	require.NoError(t, tx.QueryRow(ctx, `SELECT count(*) FROM role_space_deletion_fence_receipts`).Scan(&visible))
	require.Zero(t, visible, "non-superuser fixture must actually have hidden saved evidence")
	_, err = tx.Exec(ctx, role14SQL(t, "000014_space_deletion_fence.down.sql"))
	require.Error(t, err, "filtered empty reads must never authorize dropping saved evidence")
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, before, role14Rows(t, ctx, pool, "role_space_deletion_fence_receipts"))
}
