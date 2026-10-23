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
	require.Equal(t, expected, nativeCatalogRowJSONPreimageSQL)
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

func native51ExpectedDigestSQL(t *testing.T) string {
	t.Helper()
	raw := nativeCatalog51ParentExpression
	for _, row := range []string{"a", "t", "k"} {
		from := "(to_jsonb(" + row + "))::text"
		require.Equal(t, 1, strings.Count(raw, from))
		raw = strings.Replace(raw, from, "(row_to_json("+row+"))::text", 1)
	}
	return "SELECT sha256(convert_to((" + raw + "),'UTF8'))"
}

func TestNativeCatalog51FunctionDigestEnvelope(t *testing.T) {
	require.True(t, strings.Contains(nativeCatalogRowJSONBody, "RETURN ("+native51ExpectedDigestSQL(t)+");"),
		"the installed function must return the complete raw preimage's UTF8 SHA-256, not coerce raw text to bytea")
}

func TestNativeCatalog51OracleDigestEnvelope(t *testing.T) {
	require.True(t, native51ExpectedDigestSQL(t) == nativeCatalogRowJSONLiteralExpression,
		"bytea equality and retained seals require a digest oracle, independently of the raw preimage")
}

func TestNativeCatalog51ParentDigestEnvelope(t *testing.T) {
	want := "SELECT sha256(convert_to((" + nativeCatalog51ParentExpression + "),'UTF8'))"
	require.True(t, nativeCatalog51DigestSQL(nativeCatalog51ParentExpression) == want)
}
