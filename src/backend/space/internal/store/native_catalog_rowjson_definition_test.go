package store

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeCatalogRowJSONExpressionPreservesEveryOldProjection(t *testing.T) {
	expected := nativeCatalog51ParentExpression
	for _, row := range []string{"a", "t", "k"} {
		old := "(to_jsonb(" + row + "))::text"
		require.Equal(t, 1, strings.Count(expected, old))
		expected = strings.Replace(expected, old, "(row_to_json("+row+"))::text", 1)
	}
	require.Equal(t, expected, nativeCatalogRowJSONLiteralExpression)
	// This equality protects complete composite values, predicates, OIDs, every
	// other old field and aggregate/ordering semantics against a narrowed encoder.
}

func TestNativeCatalogRowJSONDefinitionPinsDistinctSource(t *testing.T) {
	require.Contains(t, nativeCatalogRowJSONDefinition, "LANGUAGE plpgsql")
	require.Contains(t, nativeCatalogRowJSONDefinition, "RETURNS bytea")
	require.Contains(t, nativeCatalogRowJSONDefinition, "source type qualification failed")
	require.Equal(t, nativeCatalogRowJSONBodySHA256, nativeSourceBodySHA(nativeCatalogRowJSONBody))
	for _, version := range []int{160015, 180006} {
		old, err := nativeCatalogRowJSONParentDefinition(version)
		require.NoError(t, err)
		_, want, err := nativeCatalog50Definitions(version)
		require.NoError(t, err)
		require.Equal(t, want, old, "the real original50 parent is never replaced by a candidate body")
	}
}
