package main

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/nativestartup"
)

func TestLegacyLifecycleDatabaseRefusesNativeDescriptorWithoutDatabase(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("fixed protected path exercised in owned Linux container")
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(nativestartup.DescriptorPath), 0755))
	file, err := os.OpenFile(nativestartup.DescriptorPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0444)
	require.NoError(t, err, "test owns only a newly created descriptor in its isolated container")
	require.NoError(t, file.Close())
	defer os.Remove(nativestartup.DescriptorPath)
	t.Setenv("VOICE_DATABASE_URL", "")
	store, pool, closeDatabase, enabled, err := openLifecycleDatabase(context.Background())
	require.Error(t, err)
	require.False(t, enabled)
	require.Nil(t, store)
	require.Nil(t, pool)
	closeDatabase()
}

func TestLegacyLifecycleDatabaseRefusesPersistedCoverageBeforeFallback(t *testing.T) {
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("requires owned Linux PostgreSQL fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	defer admin.Close()
	schema := "legacy_startup_" + uuid.NewString()[:8]
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	defer func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
	}()
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	seed, err := pgxpool.New(ctx, u.String())
	require.NoError(t, err)
	defer seed.Close()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, migration := range []string{"000001_room_lifecycle.up.sql", "000015_party_coverage.up.sql"} {
		body, readErr := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "migrations", "voice_db", migration))
		require.NoError(t, readErr)
		_, execErr := seed.Exec(ctx, string(body))
		require.NoError(t, execErr)
	}
	t.Setenv("VOICE_DATABASE_URL", u.String())
	store, pool, closeDatabase, enabled, err := openLifecycleDatabase(ctx)
	require.NoError(t, err, "never-begun coverage does not break admitted pre-cut legacy startup")
	require.True(t, enabled)
	require.NotNil(t, store)
	require.NotNil(t, pool)
	closeDatabase()
	op, environment, incarnation := uuid.New(), uuid.New(), uuid.New()
	_, err = seed.Exec(ctx, `INSERT INTO voice_party_coverage_operations(operation_id,environment_id,authority_incarnation,coverage_epoch,plan_bytes,plan_sha256) VALUES($1,$2,$3,1,$4,$5)`, op, environment, incarnation, []byte("negative startup fixture"), make([]byte, 32))
	require.NoError(t, err)
	_, err = seed.Exec(ctx, `INSERT INTO voice_party_coverage_head(environment_id,authority_incarnation,coverage_epoch,phase,current_operation_id,revision) VALUES($1,$2,1,'PREPARING',$3,1)`, environment, incarnation, op)
	require.NoError(t, err)
	store, pool, closeDatabase, enabled, err = openLifecycleDatabase(ctx)
	if closeDatabase != nil {
		defer closeDatabase()
	}
	require.Error(t, err, "a persisted cut cannot fall back to legacy even without ordinary roster rows")
	require.False(t, enabled)
	require.Nil(t, store)
	require.Nil(t, pool)
}
