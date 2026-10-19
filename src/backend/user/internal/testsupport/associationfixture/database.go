// Package associationfixture provides owned, port-free PostgreSQL fixtures for
// canonical User owner tests. It is never imported by production service code.
package associationfixture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type Database struct {
	Pool     *pgxpool.Pool
	Profiles []uuid.UUID
	Accounts map[uuid.UUID]uuid.UUID
}

func New(t *testing.T, count int) Database {
	t.Helper()
	require.Positive(t, count)
	dsn := os.Getenv("USER_ASSOCIATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set USER_ASSOCIATION_TEST_DSN to an owned PostgreSQL16 instance")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	name := "captureapi_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize())
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = name
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, err := root.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize())
		require.NoError(t, err)
		root.Close()
	})
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(0,false)`)
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "src", "backend", "migrations", "user_db")); err == nil {
			break
		}
		next := filepath.Dir(dir)
		require.NotEqual(t, dir, next)
		dir = next
	}
	migrations := filepath.Join(dir, "src", "backend", "migrations", "user_db")
	files, err := os.ReadDir(migrations)
	require.NoError(t, err)
	out := Database{Pool: pool, Accounts: map[uuid.UUID]uuid.UUID{}}
	var accounts []uuid.UUID
	var names []string
	for i := 0; i < count; i++ {
		id, account := uuid.New(), uuid.New()
		out.Profiles = append(out.Profiles, id)
		accounts = append(accounts, account)
		out.Accounts[id] = account
		names = append(names, fmt.Sprintf("capturefixture%d", i))
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") || file.Name() >= "000023" {
			continue
		}
		version, err := strconv.Atoi(file.Name()[:6])
		require.NoError(t, err)
		if version == 22 {
			_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) SELECT p,a,n,'1234','Capture fixture' FROM unnest($1::uuid[],$2::uuid[],$3::text[]) candidate(p,a,n)`, out.Profiles, accounts, names)
			require.NoError(t, err)
		}
		_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=$1,dirty=true`, version)
		require.NoError(t, err)
		sql, err := os.ReadFile(filepath.Join(migrations, file.Name()))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, file.Name())
		_, err = pool.Exec(ctx, `UPDATE schema_migrations SET dirty=false`)
		require.NoError(t, err)
	}
	return out
}
