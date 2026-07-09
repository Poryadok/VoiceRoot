package integrationtest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// MigrationSuffixMatches reports whether relPath ends with suffix after normalizing
// to forward slashes (Windows-safe migration chain detection).
func MigrationSuffixMatches(relPath, suffix string) bool {
	norm := strings.ReplaceAll(relPath, `\`, "/")
	want := strings.ReplaceAll(suffix, `\`, "/")
	return strings.HasSuffix(norm, want)
}

// ApplySQLFile executes a migration SQL file relative to repoRoot and applies
// follow-on migrations when known anchor files are used (encryption (docs/features/encryption.md) E2E chains).
func ApplySQLFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, repoRoot, relPath string) {
	t.Helper()
	p := filepath.Join(repoRoot, relPath)
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)

	if MigrationSuffixMatches(relPath, "messaging_db/000002_client_message_id.up.sql") {
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000004_delete_for_me.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000005_reactions.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000010_ghost_only.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000006_pins.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000007_thread_index.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000008_shared_media_indexes.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "messaging_db", "000009_e2e.up.sql"))
	}
	if MigrationSuffixMatches(relPath, "chat_db/000001_init.up.sql") {
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "chat_db", "000002_dm_requests.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "chat_db", "000005_thread_settings.up.sql"))
		ApplySQLFile(t, ctx, pool, repoRoot, filepath.Join("src", "backend", "migrations", "chat_db", "000006_e2e_enabled.up.sql"))
	}
}
