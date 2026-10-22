package store

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:embed nativecatalog/source_after50-prior.sql
var nativeObserverPrior50Definition string

func TestNativeObserver50NoopDeltaIsOnlyCurrentTargetWhere(t *testing.T) {
	_, prior, ok := strings.Cut(nativeObserverPrior50Definition, "AS $function$")
	require.True(t, ok)
	prior, _, ok = strings.Cut(prior, "$function$")
	require.True(t, ok)
	_, current, ok := strings.Cut(nativeObserver50Definition, "AS $function$")
	require.True(t, ok)
	current, _, ok = strings.Cut(current, "$function$")
	require.True(t, ok)
	const oldWhere = "   WHERE xid=r.xid AND space_id=s AND profile_id=r.profile_id;"
	const newWhere = `   WHERE xid=r.xid AND space_id=s AND profile_id=r.profile_id
    AND ((NOT ordinary_cut AND (r.old_sources->>'ordinary' IS NOT NULL AND r.old_sources->>'ordinary' IS DISTINCT FROM src->>'ordinary'))
     OR (NOT community_cut AND (r.old_sources->>'community' IS NOT NULL AND r.old_sources->>'community' IS DISTINCT FROM src->>'community')));`
	require.Equal(t, 1, strings.Count(prior, oldWhere))
	require.Equal(t, strings.Replace(prior, oldWhere, newWhere, 1), current, "all source reads/selection, UPDATE commands, frame mutation and clock observations remain byte exact")
	require.NotContains(t, newWhere, "r.ordinary_cut", "predicate uses current target state")
	require.NotContains(t, newWhere, "r.community_cut")
}

func TestNativeObserver50DefinitionPairMatchesExactMigrationBodies(t *testing.T) {
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
	oldBody := readBody("000044_native_membership_readiness.up.sql", "CREATE FUNCTION native_membership_guard.source_after()")
	newBody := readBody("000050_account_ban_coverage.up.sql", "CREATE OR REPLACE FUNCTION native_membership_guard.source_after()")
	for _, version := range []int{160000, 160014, 180000, 180006} {
		oldDefinition, newDefinition, err := nativeObserver50Definitions(version)
		require.NoError(t, err)
		require.Equal(t, 1, strings.Count(oldDefinition, oldBody), "original body bytes, including CRLF, are preserved")
		require.Equal(t, 1, strings.Count(newDefinition, newBody))
		require.Equal(t, strings.Replace(oldDefinition, oldBody, "<source body>", 1), strings.Replace(newDefinition, newBody, "<source body>", 1), "only the intended function body differs")
	}
	for _, version := range []int{0, 150000, 170000, 190000} {
		_, _, err := nativeObserver50Definitions(version)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable, "unqualified engine serialization must hold")
	}
}
