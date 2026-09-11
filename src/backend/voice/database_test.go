package main

import (
	"context"
	"net"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func TestOpenLifecycleDatabaseSourceDisabledWithoutVoiceDSN(t *testing.T) {
	t.Setenv("VOICE_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://must:not-be-used@127.0.0.1:1/wrong_db?sslmode=disable")

	store, closeDatabase, enabled, err := openLifecycleDatabase(context.Background())

	require.NoError(t, err)
	require.False(t, enabled, "missing VOICE_DATABASE_URL must keep the R22 lifecycle source disabled")
	require.Nil(t, store, "source-disabled startup must not fall back to a memory lifecycle store")
	require.NotNil(t, closeDatabase, "disabled startup must still return a safe cleanup function")
	closeDatabase()
}

func TestOpenLifecycleDatabaseRejectsInvalidConfiguredDSN(t *testing.T) {
	t.Setenv("VOICE_DATABASE_URL", "://not-a-postgres-dsn")
	t.Setenv("POSTGRES_CONNECT_TIMEOUT", "100ms")

	store, closeDatabase, enabled, err := openLifecycleDatabase(context.Background())

	require.Error(t, err)
	require.False(t, enabled)
	require.Nil(t, store)
	require.NotNil(t, closeDatabase)
	closeDatabase()
}

func TestOpenLifecycleDatabaseUsesBoundedPostgresConnectTimeout(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })

	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		close(accepted)
		defer connection.Close()
		// Keep the socket open without completing the PostgreSQL handshake. The
		// runtime constructor must return when POSTGRES_CONNECT_TIMEOUT expires.
		<-time.After(5 * time.Second)
	}()

	t.Setenv("VOICE_DATABASE_URL", "postgres://voice:voice@"+listener.Addr().String()+"/voice_db?sslmode=disable")
	t.Setenv("POSTGRES_CONNECT_TIMEOUT", "100ms")

	started := time.Now()
	store, closeDatabase, enabled, openErr := openLifecycleDatabase(context.Background())
	elapsed := time.Since(started)

	require.Error(t, openErr, "a configured database that cannot finish handshake must fail startup")
	require.False(t, enabled)
	require.Nil(t, store)
	require.NotNil(t, closeDatabase)
	closeDatabase()
	require.Less(t, elapsed, 2*time.Second, "startup must honor the repository connect timeout")
	select {
	case <-accepted:
	default:
		t.Fatal("the configured DSN was not contacted; startup must ping the pool")
	}
}

func TestOpenLifecycleDatabaseReturnsCheckedStoreAndClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
	migration := filepath.Join(repoRoot, "src", "backend", "migrations", "voice_db", "000001_room_lifecycle.up.sql")
	seedPool := integrationtest.StartPostgres(t, ctx, "voice_runtime_test", migration)

	t.Setenv("VOICE_DATABASE_URL", seedPool.Config().ConnString())
	t.Setenv("POSTGRES_CONNECT_TIMEOUT", "5s")

	store, closeDatabase, enabled, err := openLifecycleDatabase(ctx)
	require.NoError(t, err)
	require.True(t, enabled)
	require.NotNil(t, store)
	require.NotNil(t, closeDatabase)
	require.NoError(t, store.CheckSchema(ctx), "startup must return a store backed by the expected Voice schema")

	closeDatabase()
	checkCtx, checkCancel := context.WithTimeout(context.Background(), time.Second)
	defer checkCancel()
	require.Error(t, store.CheckSchema(checkCtx), "the returned cleanup function must close the lifecycle pool")
}
