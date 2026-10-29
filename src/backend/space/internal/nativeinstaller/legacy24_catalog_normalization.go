package nativeinstaller

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
)

// PG16 logical restore materializes these four known historical fast-default
// values. Full original row values are independently captured and compared.
// No other column flag, type, constraint or unexpected object is normalized.
func normalizeLegacyCatalog(raw string) ([]byte, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return nil, errors.New("legacy catalog unavailable")
	}
	d := json.NewDecoder(bytes.NewBufferString(raw))
	d.UseNumber()
	var root map[string]any
	if d.Decode(&root) != nil {
		return nil, errors.New("legacy catalog unavailable")
	}
	columns, ok := root["columns"].([]any)
	if !ok {
		return nil, errors.New("legacy catalog unavailable")
	}
	missing := map[string]any{"community_owner_authority.roster_source_revision": json.Number("0"), "space_lifecycle_aggregates.evidence_compacted": false, "space_tree_nodes.is_pinned": false, "spaces.allow_guests": true}
	for _, entry := range columns {
		col, ok := entry.([]any)
		if !ok || len(col) != 18 {
			return nil, errors.New("legacy catalog unavailable")
		}
		table, tok := col[0].(string)
		name, nok := col[1].(string)
		if !tok || !nok {
			return nil, errors.New("legacy catalog unavailable")
		}
		value, known := missing[table+"."+name]
		if known && col[13] == true && reflect.DeepEqual(col[14], []any{value}) {
			col[13] = false
			col[14] = nil
		}
	}
	checks := map[string][2]string{
		"space_tree_nodes.space_tree_nodes_kind_check": {"CHECK (kind::text = ANY (ARRAY['text_chat'::character varying, 'voice_room'::character varying]::text[]))", "CHECK (kind::text = ANY (ARRAY['text_chat'::character varying::text, 'voice_room'::character varying::text]))"},
		"spaces.spaces_entry_requirement_check":        {"CHECK (entry_requirement::text = ANY (ARRAY['none'::character varying, 'phone'::character varying, 'captcha'::character varying, 'questions'::character varying, 'manual'::character varying]::text[]))", "CHECK (entry_requirement::text = ANY (ARRAY['none'::character varying::text, 'phone'::character varying::text, 'captcha'::character varying::text, 'questions'::character varying::text, 'manual'::character varying::text]))"},
		"spaces.spaces_verification_type_check":        {"CHECK (verification_type::text = ANY (ARRAY['none'::character varying, 'personal'::character varying, 'organization'::character varying]::text[]))", "CHECK (verification_type::text = ANY (ARRAY['none'::character varying::text, 'personal'::character varying::text, 'organization'::character varying::text]))"},
		"spaces.spaces_visibility_check":               {"CHECK (visibility::text = ANY (ARRAY['public'::character varying, 'invite_only'::character varying, 'private'::character varying]::text[]))", "CHECK (visibility::text = ANY (ARRAY['public'::character varying::text, 'invite_only'::character varying::text, 'private'::character varying::text]))"},
	}
	constraints, ok := root["constraints"].([]any)
	if !ok {
		return nil, errors.New("legacy catalog unavailable")
	}
	for _, entry := range constraints {
		c, ok := entry.([]any)
		if !ok || len(c) != 6 {
			return nil, errors.New("legacy catalog unavailable")
		}
		table, tok := c[0].(string)
		name, nok := c[1].(string)
		text, sok := c[2].(string)
		if !tok || !nok || !sok {
			return nil, errors.New("legacy catalog unavailable")
		}
		if pair, found := checks[table+"."+name]; found && text == pair[1] {
			c[2] = pair[0]
		}
	}
	result, err := json.Marshal(root)
	if err != nil {
		return nil, errors.New("legacy catalog unavailable")
	}
	return result, nil
}
