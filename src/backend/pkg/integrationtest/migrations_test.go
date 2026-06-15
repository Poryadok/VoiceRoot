package integrationtest

import (
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
