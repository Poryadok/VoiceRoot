package roomlifecycle

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

const r22VoiceMigrationBase = "000001_room_lifecycle"

func r22VoiceRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func r22VoiceMigrationPath(t *testing.T, direction string) string {
	t.Helper()
	return filepath.Join(
		r22VoiceRepoRoot(t),
		"src", "backend", "migrations", "voice_db",
		r22VoiceMigrationBase+"."+direction+".sql",
	)
}

func r22ReadVoiceMigration(t *testing.T, direction string) string {
	t.Helper()
	raw, err := os.ReadFile(r22VoiceMigrationPath(t, direction))
	require.NoError(t, err, "R22.2 Voice migration must exist before schema tests can run")
	return string(raw)
}

func r22StartVoicePostgres(t *testing.T, ctx context.Context, databaseName string) *pgxpool.Pool {
	t.Helper()
	migrationPath := r22VoiceMigrationPath(t, "up")
	_, err := os.Stat(migrationPath)
	require.NoError(t, err, "R22.2 Voice UP migration must exist before starting PostgreSQL")
	return integrationtest.StartPostgres(t, ctx, databaseName, migrationPath)
}

func r22ConstraintDefinitions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, table string) []string {
	t.Helper()
	rows, err := pool.Query(ctx, `
SELECT pg_get_constraintdef(c.oid)
FROM pg_constraint c
JOIN pg_class r ON r.oid = c.conrelid
JOIN pg_namespace n ON n.oid = r.relnamespace
WHERE n.nspname = current_schema() AND r.relname = $1 AND c.contype = 'c'
ORDER BY c.conname`, table)
	require.NoError(t, err)
	defer rows.Close()

	var definitions []string
	for rows.Next() {
		var definition string
		require.NoError(t, rows.Scan(&definition))
		definitions = append(definitions, strings.ToLower(strings.ReplaceAll(definition, " ", "")))
	}
	require.NoError(t, rows.Err())
	return definitions
}

func r22RequireColumnCheck(t *testing.T, definitions []string, column string, fragments ...string) {
	t.Helper()
	for _, definition := range definitions {
		if !strings.Contains(definition, strings.ToLower(column)) {
			continue
		}
		matches := true
		for _, fragment := range fragments {
			if !strings.Contains(definition, strings.ToLower(strings.ReplaceAll(fragment, " ", ""))) {
				matches = false
				break
			}
		}
		if matches {
			return
		}
	}
	t.Fatalf("column %s has no CHECK containing %v; constraints: %v", column, fragments, definitions)
}
