package mmadmissionwire

import "testing"

func TestStrictAuthorityJSON(t *testing.T) {
	type value struct {
		Version uint32 `json:"protocol_version"`
		Nested  struct {
			ID string `json:"id"`
		} `json:"nested"`
	}
	for _, raw := range []string{
		`{"protocol_version":1,"protocol_version":2}`,
		`{"protocol_version":1,"nested":{"id":"a","id":"b"}}`,
		`{"protocol_version":1,"unknown":true}`,
		`{"protocol_version":1} {"protocol_version":2}`,
		`{"protocol_version":1,"PROTOCOL_VERSION":2,"nested":{"id":"a"}}`,
		`{"protocol_version":1,"nested":{"id":"a","ID":"b"}}`,
		`{"protocol_version":1,"nested":{"ID":"b","id":"a"}}`,
		`{"protocol_version":1,"nested":{"id":"a"},"NESTED":{"id":"b"}}`,
		`{"protocol_version":1,"Nested":{"id":"a"}}`,
	} {
		var v value
		if Decode([]byte(raw), &v) == nil {
			t.Errorf("accepted ambiguous authority: %s", raw)
		}
	}
	var v value
	if err := Decode([]byte(`{"protocol_version":1,"nested":{"id":"a"}}`), &v); err != nil {
		t.Fatal(err)
	}
}

func TestStrictJSONTagsInsideWrappersAndSlices(t *testing.T) {
	type item struct {
		ID string `json:"id"`
	}
	type envelope struct {
		Payload []item `json:"payload"`
	}
	for _, raw := range []string{
		`{"payload":[{"id":"a"}],"PAYLOAD":[{"id":"b"}]}`,
		`{"payload":[{"id":"a","ID":"b"}]}`,
		`{"Payload":[{"id":"a"}]}`,
	} {
		var v envelope
		if Decode([]byte(raw), &v) == nil {
			t.Errorf("accepted alias in wrapper: %s", raw)
		}
	}
	var v envelope
	if e := Decode([]byte(`{"payload":[{"id":"a"}]}`), &v); e != nil || v.Payload[0].ID != "a" {
		t.Fatal("canonical wrapper rejected", e)
	}
}
