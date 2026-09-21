package integrationtest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserDBMigrationFiles_IncludesPrivacyShowLastSeenMigration(t *testing.T) {
	require.Contains(t, UserDBMigrationFiles, "000013_privacy_show_last_seen.up.sql",
		"integration fixtures must apply the same additive schema used by the privacy store")
	require.Contains(t, UserDBMigrationFiles, "000014_profile_custom_status.up.sql",
		"integration fixtures must apply the same durable profile schema used by UpdateProfile")
	require.Contains(t, UserDBMigrationFiles, "000016_search_profile_journal.up.sql",
		"integration fixtures must apply the User-authoritative search projection schema")
}
