package store

import (
	"os"
	"regexp"
	"strings"
	"testing"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

// The stored binding is the canonical credential hash, not an unrelated digest.
func TestMembershipMigrationAcceptsCanonicalCredentialHashes(t *testing.T) {
	raw, err := os.ReadFile("../../../migrations/bot_db/000008_space_membership_intents.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	hash, err := principal.RequestHash(&spacev1.AddBotMemberRequest{})
	if err != nil {
		t.Fatal(err)
	}
	constraints := regexp.MustCompile(`(?:request_hash|inputs_hash|initiating_request_hash) ~ '([^']+)'`).FindAllStringSubmatch(string(raw), -1)
	if len(constraints) != 5 {
		t.Fatalf("expected all five hash constraints, got %d", len(constraints))
	}
	for _, constraint := range constraints {
		if !regexp.MustCompile(constraint[1]).MatchString(hash) {
			t.Errorf("schema rejects canonical hash %q with %s", hash, constraint[1])
		}
		if regexp.MustCompile(constraint[1]).MatchString(strings.TrimPrefix(hash, "sha256:")) {
			t.Error("schema accepts unqualified digest")
		}
	}
}
