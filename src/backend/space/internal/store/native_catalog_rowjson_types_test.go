package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeCatalogRowJSONQualificationRequiresExactEvidence(t *testing.T) {
	// A finite type screen alone cannot authorize a new serialization profile.
	q := nativeCatalogRowJSONQualification{}
	require.Error(t, q.validate())
	q.TypeProofSHA256 = nativeCatalogRowJSONTypeProofSHA256
	q.TypeExpectationSHA256 = nativeCatalogRowJSONTypeExpectationSHA256
	q.DefinitionSHA256 = nativeCatalogRowJSONDefinitionSHA256
	require.Error(t, q.validate(), "raw candidate/session stability is independently required")
	q.StabilitySHA256 = "caller-selected"
	require.Error(t, q.validate())
}

func TestNativeCatalogRowJSONTypesAreFiniteByEngine(t *testing.T) {
	for _, version := range []int{160015, 180006} {
		sql, err := nativeCatalogRowJSONTypes(version)
		require.NoError(t, err)
		require.Contains(t, sql, "pg_attribute")
		require.Contains(t, sql, "pg_trigger")
		require.Contains(t, sql, "pg_constraint")
		require.Contains(t, sql, "array_send")
		require.Contains(t, sql, "ARRAY[true]::pg_catalog.bool[]", "M6 historical true must not become M16 current false")
		require.Contains(t, sql, "ARRAY[0]::pg_catalog.int8[]")
		require.Contains(t, sql, "IS DISTINCT FROM TRUE")
	}
	for _, version := range []int{0, 150000, 170000, 190000} {
		_, err := nativeCatalogRowJSONTypes(version)
		require.ErrorIs(t, err, ErrMemberAuthorityUnavailable)
	}
}
