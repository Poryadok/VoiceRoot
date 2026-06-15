package integrationtest

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Batch E2E-A audit: shared migration suffix helper for Windows path chains (docs/TODO.md).
func TestMigrationSuffixMatches_WindowsPath(t *testing.T) {
	winPath := `src\backend\migrations\messaging_db\000002_client_message_id.up.sql`
	require.True(t, MigrationSuffixMatches(winPath, "messaging_db/000002_client_message_id.up.sql"))
	require.False(t, MigrationSuffixMatches(winPath, "chat_db/000001_init.up.sql"))
}

func TestMigrationSuffixMatches_UnixPath(t *testing.T) {
	unixPath := "src/backend/migrations/chat_db/000001_init.up.sql"
	require.True(t, MigrationSuffixMatches(unixPath, "chat_db/000001_init.up.sql"))
	require.True(t, MigrationSuffixMatches(unixPath, "migrations/chat_db/000001_init.up.sql"))
}

func TestMigrationSuffixMatches_NormalizesBackslashes(t *testing.T) {
	mixed := `migrations\chat_db/000006_e2e_enabled.up.sql`
	require.True(t, MigrationSuffixMatches(mixed, "chat_db/000006_e2e_enabled.up.sql"))
}

// Phase 15 E2E-B red: incremental migration chains must include Phase 15 DDL snippets.
func TestPhase15IncrementalSnippets_IncludeE2EDDL(t *testing.T) {
	root := migrationsTestRepoRoot(t)
	cases := []struct {
		relPath string
		needle  string
	}{
		{
			relPath: filepath.Join("src", "backend", "migrations", "chat_db", "000006_e2e_enabled.up.sql"),
			needle:  "e2e_enabled",
		},
		{
			relPath: filepath.Join("src", "backend", "migrations", "messaging_db", "000009_e2e.up.sql"),
			needle:  "e2e_prekey_bundles",
		},
		{
			relPath: filepath.Join("src", "backend", "migrations", "messaging_db", "000009_e2e.up.sql"),
			needle:  "is_e2e",
		},
		{
			relPath: filepath.Join("src", "backend", "migrations", "auth_db", "000005_e2e_key_backup.up.sql"),
			needle:  "e2e_key_backups",
		},
	}
	for _, tc := range cases {
		t.Run(tc.needle, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(root, tc.relPath))
			require.NoError(t, err)
			require.Contains(t, strings.ToLower(string(body)), strings.ToLower(tc.needle))
		})
	}
}

// Phase 15 E2E-B red: Auth Flyway V4 must mirror golang-migrate auth_db/000005.
func TestPhase15AuthFlywayV4_ParityWithGolangMigrate(t *testing.T) {
	root := migrationsTestRepoRoot(t)
	flywayPath := filepath.Join(
		root,
		"src", "backend", "auth", "src", "main", "resources", "db", "migration",
		"V4__e2e_key_backups.sql",
	)
	golangPath := filepath.Join(root, "src", "backend", "migrations", "auth_db", "000005_e2e_key_backup.up.sql")

	flywayBody, err := os.ReadFile(flywayPath)
	require.NoError(t, err, "expected Flyway V4 migration for e2e_key_backups")
	golangBody, err := os.ReadFile(golangPath)
	require.NoError(t, err)

	require.Contains(t, strings.ToLower(string(flywayBody)), "e2e_key_backups")
	require.Contains(t, strings.ToLower(string(golangBody)), "e2e_key_backups")
}

func migrationsTestRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}
