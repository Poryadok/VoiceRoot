package store

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed nativecatalog/catalog44-text-expression.sql
var nativeCatalog44TextExpression string

//go:embed nativecatalog/catalog50-expression.sql
var nativeCatalog50TextExpression string

func TestNativeCatalog50DefinitionPairMatchesExactMigration(t *testing.T) {
	readBody := func(file, declaration string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db", file))
		require.NoError(t, err)
		_, tail, ok := strings.Cut(string(raw), declaration)
		require.True(t, ok)
		_, tail, ok = strings.Cut(tail, "AS $$")
		require.True(t, ok)
		body, _, ok := strings.Cut(tail, "$$;")
		require.True(t, ok)
		return body
	}
	oldBody := readBody("000044_native_membership_readiness.up.sql", "CREATE FUNCTION native_membership_guard.catalog()")
	newBody := readBody("000050_account_ban_coverage.up.sql", "CREATE OR REPLACE FUNCTION native_membership_guard.catalog()")
	require.Contains(t, newBody, nativeCatalog50TextExpression)
	require.Contains(t, nativeCatalog44TextExpression, "jsonb_build_object(")
	require.NotContains(t, nativeCatalog50TextExpression, "::jsonb", "no full-object parse back")
	for _, version := range []int{160000, 160014, 180000, 180006} {
		oldDefinition, newDefinition, err := nativeCatalog50Definitions(version)
		require.NoError(t, err)
		require.Equal(t, 1, strings.Count(oldDefinition, oldBody))
		require.Equal(t, 1, strings.Count(newDefinition, newBody))
		require.Equal(t, strings.Replace(oldDefinition, oldBody, "<body>", 1), strings.Replace(newDefinition, newBody, "<body>", 1))
	}
	for _, version := range []int{0, 150000, 170000, 190000} {
		_, _, err := nativeCatalog50Definitions(version)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	}
	// The differential oracle is the immutable original expression, with no FK
	// exclusions or function substitutions and only SQL whitespace normalized.
	normalize := func(s string) string { return strings.Join(strings.Fields(s), " ") }
	require.Equal(t, normalize(oldBody), normalize(nativeCatalog44LiteralExpression))
}
