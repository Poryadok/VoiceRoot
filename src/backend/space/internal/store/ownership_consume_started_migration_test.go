package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOwnershipConsumeStartedMigration_FencesRecoveryAndHasRollback(t *testing.T) {
	root := repoRoot(t)
	upPath := filepath.Join(root, "src", "backend", "migrations", "space_db", "000018_ownership_consume_started.up.sql")
	downPath := filepath.Join(root, "src", "backend", "migrations", "space_db", "000018_ownership_consume_started.down.sql")
	up, err := os.ReadFile(upPath)
	require.NoError(t, err)
	down, err := os.ReadFile(downPath)
	require.NoError(t, err)
	upSQL, downSQL := strings.ToLower(string(up)), strings.ToLower(string(down))
	require.Contains(t, upSQL, "consume_started")
	require.Contains(t, upSQL, "ownership_journal_state_check")
	require.Contains(t, downSQL, "update ownership_journal set state='reserved' where state='consume_started'")
	require.Contains(t, downSQL, "drop constraint ownership_journal_consume_started_has_no_receipt")
}
