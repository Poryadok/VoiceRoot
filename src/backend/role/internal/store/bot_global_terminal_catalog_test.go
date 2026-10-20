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
	"voice/backend/pkg/integrationtest"
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
