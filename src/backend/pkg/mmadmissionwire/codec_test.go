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
