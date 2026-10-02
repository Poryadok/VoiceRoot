package store

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRoleMigrationSourceHasOneMigrationPerVersion(t *testing.T) {
	directory := filepath.Join(repoRoot(t), "src", "backend", "migrations", "role_db")
	files, err := os.ReadDir(directory)
	require.NoError(t, err)
	up, down := map[int64]string{}, map[int64]string{}
	for _, file := range files {
		name := file.Name()
		var versions map[int64]string
		switch {
		case strings.HasSuffix(name, ".up.sql"):
			versions = up
		case strings.HasSuffix(name, ".down.sql"):
			versions = down
		default:
			continue
		}
		version, err := strconv.ParseInt(strings.SplitN(name, "_", 2)[0], 10, 64)
		require.NoError(t, err)
		require.Positive(t, version)
		if previous, exists := versions[version]; exists {
			t.Fatalf("migration version %d collides: %s and %s", version, previous, name)
		}
		versions[version] = name
	}
	for version, name := range up {
		if version <= 3 { // Existing baseline migrations are intentionally up-only.
			continue
		}
		require.Contains(t, down, version, "rollback contract missing for %s", name)
	}
}
