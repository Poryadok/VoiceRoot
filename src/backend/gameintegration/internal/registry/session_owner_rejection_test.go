package registry

import "testing"

func TestPermanentOwnerFailureCodeAllowList(t *testing.T) {
	for _, test := range []struct {
		stage, category, want string
	}{
		{"chat_create", "operation_conflict", "OWNER_CHAT_CREATE_OPERATION_CONFLICT"},
		{"chat_roster", "resource_missing", "OWNER_CHAT_ROSTER_RESOURCE_MISSING"},
		{"voice_provision", "operation_conflict", "OWNER_VOICE_PROVISION_OPERATION_CONFLICT"},
		{"role_apply", "terminal_revoked", "OWNER_ROLE_APPLY_TERMINAL_REVOKED"},
		{"role_apply", "resource_missing", ""},
		{"chat_create", "resource_missing", ""},
		{"unknown", "operation_conflict", ""},
	} {
		t.Run(test.stage+"/"+test.category, func(t *testing.T) {
			if got := permanentOwnerFailureCode(test.stage, test.category); got != test.want {
				t.Fatalf("permanentOwnerFailureCode(%q, %q) = %q, want %q", test.stage, test.category, got, test.want)
			}
		})
	}
}
