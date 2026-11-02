package nativeinstaller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLegacyCatalogNormalizationIsLimitedToKnownPG16RestoreForms(t *testing.T) {
	columns := []any{}
	for _, item := range []struct {
		table, name string
		missing     any
	}{
		{"community_owner_authority", "roster_source_revision", 0},
		{"space_lifecycle_aggregates", "evidence_compacted", false},
		{"space_tree_nodes", "is_pinned", false},
		{"spaces", "allow_guests", true},
	} {
		column := make([]any, 18)
		column[0], column[1], column[13], column[14] = item.table, item.name, true, []any{item.missing}
		columns = append(columns, column)
	}
	canonical := `CHECK (visibility::text = ANY (ARRAY['public'::character varying, 'invite_only'::character varying, 'private'::character varying]::text[]))`
	restored := `CHECK (visibility::text = ANY (ARRAY['public'::character varying::text, 'invite_only'::character varying::text, 'private'::character varying::text]))`
	root := map[string]any{"columns": columns, "constraints": []any{[]any{"spaces", "spaces_visibility_check", canonical, true, false, false}}, "other": "retained"}
	encode := func() []byte { raw, err := json.Marshal(root); require.NoError(t, err); return raw }
	direct, err := normalizeLegacyCatalog(string(encode()))
	require.NoError(t, err)
	for _, entry := range columns {
		column := entry.([]any)
		column[13], column[14] = false, nil
	}
	root["constraints"].([]any)[0].([]any)[2] = restored
	clone, err := normalizeLegacyCatalog(string(encode()))
	require.NoError(t, err)
	require.Equal(t, direct, clone)
	column := columns[3].([]any)
	column[13], column[14] = true, []any{false}
	wrong, err := normalizeLegacyCatalog(string(encode()))
	require.NoError(t, err)
	require.NotEqual(t, direct, wrong, "a different historical missing value is not normalized")
	column[13], column[14] = false, nil
	root["constraints"].([]any)[0].([]any)[2] = restored + " "
	wrong, err = normalizeLegacyCatalog(string(encode()))
	require.NoError(t, err)
	require.NotEqual(t, direct, wrong, "unknown constraint text is not normalized")
}
