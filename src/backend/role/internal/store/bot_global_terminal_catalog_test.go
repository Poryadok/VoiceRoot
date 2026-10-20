package store

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/authoritysource"
	"voice/backend/pkg/integrationtest"
	authorityv1 "voice/backend/pkg/pb/voice/authority/v1"
	"voice/backend/role/internal/memberauthority"
)

func TestRole26SourceMigration(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "25")
	f.Run(true, "goto", "26")
	var relation string
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT 'public.role_bot_global_terminals'::regclass::text").Scan(&relation))
	require.Equal(t, "role_bot_global_terminals", relation)
	f.Run(true, "down", "1")
	f.Run(true, "up", "1")
}

func role26Fixture(t *testing.T) integrationtest.SourceMigrationFixture {
	t.Helper()
	f := integrationtest.NewSourceMigrationFixture(t, context.Background(), filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "26")
	return f
}

func role26Serving(t *testing.T, admin *pgxpool.Pool) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	name, password := "role26_"+strings.ReplaceAll(uuid.NewString(), "-", ""), uuid.NewString()
	identifier := pgx.Identifier{name}.Sanitize()
	_, err := admin.Exec(ctx, "CREATE ROLE "+identifier+" LOGIN PASSWORD '"+password+"' NOSUPERUSER NOBYPASSRLS NOCREATEDB NOCREATEROLE NOREPLICATION")
	require.NoError(t, err)
	_, err = admin.Exec(ctx, "GRANT USAGE ON SCHEMA public TO "+identifier+"; GRANT SELECT ON ALL TABLES IN SCHEMA public TO "+identifier+"; GRANT INSERT ON public.role_bot_global_terminals TO "+identifier)
	require.NoError(t, err)
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User, cfg.ConnConfig.Password = name, password
	app, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(app.Close)
	var actual string
	require.NoError(t, app.QueryRow(ctx, "SELECT session_user").Scan(&actual))
	require.Equal(t, name, actual, "actual independent LOGIN, never SET ROLE simulation")
	return app
}

func role26Check(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	return (&RoleStore{Pool: pool, tx: tx}).checkBotGlobalTerminalCatalog(ctx)
}

func TestRole26CatalogServingACL(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	app := role26Serving(t, f.Pool)
	require.NoError(t, role26Check(ctx, app), "qualified serving positive beside every negative")
	require.ErrorIs(t, role26Check(ctx, f.Pool), ErrScopeUnavailable, "owner/superuser is not the serving principal")
	var user string
	require.NoError(t, app.QueryRow(ctx, "SELECT current_user").Scan(&user))
	id := pgx.Identifier{user}.Sanitize()
	for _, privilege := range []string{"UPDATE", "DELETE", "TRUNCATE", "TRIGGER", "REFERENCES"} {
		t.Run(privilege, func(t *testing.T) {
			_, err := f.Pool.Exec(ctx, "GRANT "+privilege+" ON public.role_bot_global_terminals TO "+id)
			require.NoError(t, err)
			require.ErrorIs(t, role26Check(ctx, app), ErrScopeUnavailable)
			_, err = f.Pool.Exec(ctx, "REVOKE "+privilege+" ON public.role_bot_global_terminals FROM "+id)
			require.NoError(t, err)
			require.NoError(t, role26Check(ctx, app))
		})
	}
	for _, c := range []struct{ name, change, undo string }{
		{"missing_root_unique", "ALTER TABLE public.role_bot_global_terminals DROP CONSTRAINT role_bot_global_terminals_decision_id_key", "ALTER TABLE public.role_bot_global_terminals ADD CONSTRAINT role_bot_global_terminals_decision_id_key UNIQUE(decision_id)"},
		{"extra_personal_column", "ALTER TABLE public.role_bot_global_terminals ADD COLUMN actor_profile_id UUID", "ALTER TABLE public.role_bot_global_terminals DROP COLUMN actor_profile_id"},
		{"extra_guard", "CREATE TRIGGER role26_extra BEFORE DELETE ON public.role_bot_global_terminals FOR EACH STATEMENT EXECUTE FUNCTION public.role_bot_removal_immutable()", "DROP TRIGGER role26_extra ON public.role_bot_global_terminals"},
		{"clock_default", "ALTER TABLE public.role_bot_global_terminals ALTER COLUMN registered_at SET DEFAULT '2000-01-01'::timestamptz", "ALTER TABLE public.role_bot_global_terminals ALTER COLUMN registered_at SET DEFAULT clock_timestamp()"},
		{"public_acl", "GRANT SELECT ON public.role_bot_global_terminals TO PUBLIC", "REVOKE ALL ON public.role_bot_global_terminals FROM PUBLIC"},
		{"schema_create", "GRANT CREATE ON SCHEMA public TO " + id, "REVOKE CREATE ON SCHEMA public FROM " + id},
		{"disabled_guard", "ALTER TABLE public.role_bot_global_terminals DISABLE TRIGGER role_bot_global_terminals_guard", "ALTER TABLE public.role_bot_global_terminals ENABLE TRIGGER role_bot_global_terminals_guard"},
		{"rls", "ALTER TABLE public.role_bot_global_terminals ENABLE ROW LEVEL SECURITY", "ALTER TABLE public.role_bot_global_terminals DISABLE ROW LEVEL SECURITY"},
		{"dirty", "UPDATE public.schema_migrations SET dirty=true", "UPDATE public.schema_migrations SET dirty=false"},
		{"future", "UPDATE public.schema_migrations SET version=27", "UPDATE public.schema_migrations SET version=26"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := f.Pool.Exec(ctx, c.change)
			require.NoError(t, err)
			require.ErrorIs(t, role26Check(ctx, app), ErrScopeUnavailable)
			_, err = f.Pool.Exec(ctx, c.undo)
			require.NoError(t, err)
			require.NoError(t, role26Check(ctx, app))
		})
	}
	bot, decision, receipt := uuid.New(), uuid.New(), uuid.New()
	_, err := app.Exec(ctx, "INSERT INTO public.role_bot_global_terminals(bot_id,decision_id,receipt_id) VALUES($1,$2,$3)", bot, decision, receipt)
	require.NoError(t, err)
	// Administrator setup probes the immutable guard itself, not just app ACL.
	for _, query := range []string{"UPDATE public.role_bot_global_terminals SET registered_at=registered_at", "DELETE FROM public.role_bot_global_terminals", "TRUNCATE public.role_bot_global_terminals"} {
		_, err = f.Pool.Exec(ctx, query)
		require.Error(t, err)
	}
	var columns []string
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT array_agg(column_name::text ORDER BY ordinal_position) FROM information_schema.columns WHERE table_schema='public' AND table_name='role_bot_global_terminals'").Scan(&columns))
	require.Equal(t, []string{"bot_id", "decision_id", "receipt_id", "registered_at"}, columns, "no actor/Space/personal digest payload")
}

func TestRole26RollbackRetainedAndOldSnapshot(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/role_db/000026_bot_global_terminal.down.sql"))
	require.NoError(t, err)
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(string(down)), "COMMIT;"), "BEGIN;"))
	old, err := f.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var before int
	require.NoError(t, old.QueryRow(ctx, "SELECT count(*) FROM public.role_bot_global_terminals").Scan(&before))
	require.Zero(t, before)
	_, err = f.Pool.Exec(ctx, "INSERT INTO public.role_bot_global_terminals(bot_id,decision_id,receipt_id) VALUES($1,$2,$3)", uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	_, err = old.Exec(ctx, body)
	require.ErrorContains(t, err, "READ COMMITTED")
	require.NoError(t, old.Rollback(ctx))
	rc, err := f.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer rc.Rollback(ctx)
	_, err = rc.Exec(ctx, body)
	require.ErrorContains(t, err, "evidence prevents rollback")
	require.NoError(t, rc.Rollback(ctx))
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM public.role_bot_global_terminals").Scan(&count))
	require.Equal(t, 1, count)
}

func TestRole26RollbackWaitsForFirstWriter(t *testing.T) {
	ctx := context.Background()
	f := role26Fixture(t)
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/role_db/000026_bot_global_terminal.down.sql"))
	require.NoError(t, err)
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(string(down)), "COMMIT;"), "BEGIN;"))
	writer, err := f.Pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	_, err = writer.Exec(ctx, "INSERT INTO public.role_bot_global_terminals(bot_id,decision_id,receipt_id) VALUES($1,$2,$3)", uuid.New(), uuid.New(), uuid.New())
	require.NoError(t, err)
	rc, err := f.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer rc.Rollback(ctx)
	var pid int
	require.NoError(t, rc.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	done := make(chan error, 1)
	go func() { _, err := rc.Exec(ctx, body); done <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := f.Pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE pid=$1 AND wait_event_type='Lock')", pid).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "actual DOWN waits behind uncommitted first writer")
	require.NoError(t, writer.Commit(ctx))
	select {
	case err := <-done:
		require.ErrorContains(t, err, "evidence prevents rollback")
	case <-time.After(5 * time.Second):
		t.Fatal("DOWN did not settle after writer commit")
	}
	require.NoError(t, rc.Rollback(ctx))
	var count int
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM public.role_bot_global_terminals").Scan(&count))
	require.Equal(t, 1, count)
}

func TestRole26UpRejectsUnknownSourceAndOldSnapshot(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "25")
	up, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/role_db/000026_bot_global_terminal.up.sql"))
	require.NoError(t, err)
	body := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.TrimSpace(string(up)), "COMMIT;"), "BEGIN;"))
	old, err := f.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var count int
	require.NoError(t, old.QueryRow(ctx, "SELECT count(*) FROM public.role_bot_global_removals").Scan(&count))
	_, err = old.Exec(ctx, body)
	require.ErrorContains(t, err, "READ COMMITTED")
	require.NoError(t, old.Rollback(ctx))
	_, err = f.Pool.Exec(ctx, "INSERT INTO public.role_bot_actor_bindings(bot_id,actor_profile_id) VALUES($1,$2)", uuid.New(), uuid.New())
	require.NoError(t, err)
	// Corrupt only the disposable database's old guard, never published25 SQL.
	_, err = f.Pool.Exec(ctx, "ALTER TABLE public.role_bot_global_removals DISABLE TRIGGER role_bot_global_removals_guard")
	require.NoError(t, err)
	f.Run(false, "goto", "26")
	var absent bool
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT to_regclass('public.role_bot_global_terminals') IS NULL").Scan(&absent))
	require.True(t, absent, "failed preflight creates no partial terminal storage")
	require.NoError(t, f.Pool.QueryRow(ctx, "SELECT count(*) FROM public.role_bot_actor_bindings").Scan(&count))
	require.Equal(t, 1, count, "old identity/fence evidence is never erased by forward failure")
}

func TestRole26NativeAndSource2ServingParity(t *testing.T) {
	ctx := context.Background()
	f := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/role_db"))
	f.Run(true, "goto", "25")
	admin := &RoleStore{Pool: f.Pool}
	space, owner, profile := uuid.New(), uuid.New(), uuid.New()
	require.NoError(t, admin.BootstrapSpaceRoles(ctx, space, owner))
	binding := ensureMemberBinding(space, profile, 1)
	receipt, err := admin.PrepareDefaultMemberRole(ctx, binding)
	require.NoError(t, err)
	f.Run(true, "goto", "26")
	app := role26Serving(t, f.Pool)
	s := &RoleStore{Pool: app}
	fresh := func() context.Context {
		return qualifiedMemberContext(t, ctx, space, profile, owner, binding.AdmissionOperationID, 1, receipt.ReceiptSHA256)
	}
	native, err := s.GetQualifiedMemberRoles(fresh(), space, profile)
	require.NoError(t, err)
	require.Len(t, native, 1)
	require.NoError(t, s.CheckAuthoritySourceSchema(ctx), "own source registration must select state2 for actual26, not silently reject at startup")
	snapshot, err := s.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 2, SpaceId: space.String()})
	require.NoError(t, err)
	facts, err := authoritysource.DecodeRoleStateV2(snapshot.CanonicalState)
	require.NoError(t, err)
	observation, ok := memberauthority.FromContext(fresh(), space.String(), profile.String())
	require.True(t, ok)
	fold, err := authoritysource.QualifyRoleMember(facts, observation, profile.String(), "", "", time.Now())
	require.NoError(t, err)
	mask, err := s.GetQualifiedEffectiveMask(fresh(), space, profile, nil, nil)
	require.NoError(t, err)
	require.Equal(t, native[0].ID.String(), fold.Roles[0].ID)
	require.Equal(t, mask, fold.EffectiveMask)
	_, err = s.ReadAuthoritySnapshot(ctx, &authorityv1.SourceScope{SchemaVersion: 1, SpaceId: space.String()})
	require.Error(t, err, "state1 is never widened to26")
}
