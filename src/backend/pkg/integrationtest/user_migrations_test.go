package integrationtest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserDBMigrationFiles_IncludesPrivacyShowLastSeenMigration(t *testing.T) {
	require.Contains(t, UserDBMigrationFiles, "000013_privacy_show_last_seen.up.sql",
		"integration fixtures must apply the same additive schema used by the privacy store")
}
