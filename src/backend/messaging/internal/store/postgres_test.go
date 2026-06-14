package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startPostgresForStoreTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "db", "")
}

func applySQLFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, relPath string) {
	t.Helper()
	p := filepath.Join(repoRoot(t), relPath)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)
	if strings.HasSuffix(relPath, filepath.Join("messaging_db", "000002_client_message_id.up.sql")) {
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000004_delete_for_me.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000005_reactions.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000006_ghost_only.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000006_pins.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000007_thread_index.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000008_shared_media_indexes.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000009_e2e.up.sql"))
	}
	if strings.HasSuffix(relPath, filepath.Join("chat_db", "000001_init.up.sql")) {
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000002_dm_requests.up.sql"))
		applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000006_e2e_enabled.up.sql"))
	}
}

func seedMessagingSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
}

func seedChatSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
}
