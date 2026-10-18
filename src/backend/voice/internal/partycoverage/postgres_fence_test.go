package partycoverage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

func TestRetirePostgresRoleStopsPausedWriterAndRestart(t *testing.T) {
	dsn := testPostgresDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer admin.Close(ctx)
	suffix := uuid.NewString()[:8]
	role := "coverage_old_" + suffix
	table := "coverage_writer_" + suffix
	_, err = admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD 'isolated-test-only'")
	require.NoError(t, err)
	defer admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{role}.Sanitize())
	_, err = admin.Exec(ctx, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+"(n int)")
	require.NoError(t, err)
	defer admin.Exec(context.Background(), "DROP TABLE "+pgx.Identifier{table}.Sanitize())
	_, err = admin.Exec(ctx, "GRANT SELECT,INSERT ON "+pgx.Identifier{table}.Sanitize()+" TO "+pgx.Identifier{role}.Sanitize())
	require.NoError(t, err)
	cfg, err := pgx.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.User = role
	cfg.Password = "isolated-test-only"
	old, err := pgx.ConnectConfig(ctx, cfg)
	require.NoError(t, err)
	defer old.Close(ctx)
	tx, err := old.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "INSERT INTO "+pgx.Identifier{table}.Sanitize()+" VALUES(1)")
	require.NoError(t, err)
	require.NoError(t, RetirePostgresRole(ctx, admin, role))
	require.Error(t, tx.Commit(ctx), "already accepted transaction must not cross cut")
	var n int
	require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n))
	require.Zero(t, n)
	restarted, err := pgx.ConnectConfig(ctx, cfg)
	if restarted != nil {
		restarted.Close(ctx)
	}
	require.Error(t, err, "old binary restart must be denied")
	require.NoError(t, RetirePostgresRole(ctx, admin, role), "exact retry revalidates durable fence")
}

func TestRetirePostgresRoleRejectsAuthorityRoles(t *testing.T) {
	dsn := testPostgresDSN(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	require.NoError(t, err)
	defer admin.Close(ctx)
	require.Error(t, RetirePostgresRole(ctx, admin, admin.Config().User))
	require.Error(t, RetirePostgresRole(ctx, admin, ""))
}
