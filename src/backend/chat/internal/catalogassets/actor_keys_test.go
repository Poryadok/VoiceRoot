package catalogassets

import (
	"bytes"
	"github.com/google/uuid"
	"testing"
)

func TestCatalogActorKeyringSeparatesPurposesAndRetainsHistoricalAliases(t *testing.T) {
	first := bytes.Repeat([]byte{1}, 32)
	second := bytes.Repeat([]byte{2}, 32)
	old, err := NewActorKeyring("old", map[string][]byte{"old": first})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.New()
	before, err := old.aliases(1, actor)
	if err != nil || len(before) != 1 {
		t.Fatalf("old aliases=%d err=%v", len(before), err)
	}
	first[0] = 99
	after, err := old.aliases(1, actor)
	if err != nil || before[0] != after[0] {
		t.Fatal("constructor retained mutable secret input")
	}
	rotated, err := NewActorKeyring("new", map[string][]byte{"new": second, "old": bytes.Repeat([]byte{1}, 32)})
	if err != nil {
		t.Fatal(err)
	}
	aliases, err := rotated.aliases(1, actor)
	if err != nil || len(aliases) != 2 || aliases[1] != before[0] {
		t.Fatal("key rotation discarded historical actor alias")
	}
	profile, err := rotated.aliases(2, actor)
	if err != nil || profile[0].digest == aliases[0].digest {
		t.Fatal("account/profile purpose collision")
	}
	for _, tc := range []struct {
		active string
		keys   map[string][]byte
	}{{"missing", map[string][]byte{"one": second}}, {"bad id", map[string][]byte{"bad id": second}}, {"one", map[string][]byte{"one": {1}}}, {"", nil}} {
		if _, err := NewActorKeyring(tc.active, tc.keys); err == nil {
			t.Fatal("accepted invalid actor keyring")
		}
	}
	if _, err := rotated.aliases(1, uuid.Nil); err == nil {
		t.Fatal("accepted nil actor")
	}
}
