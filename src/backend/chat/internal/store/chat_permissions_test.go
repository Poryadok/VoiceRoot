package store

import "testing"

func TestStandalonePermissionsKeepCapabilitiesDistinct(t *testing.T) {
	for _, tc := range []struct {
		name, role          string
		additions, explicit bool
		post, add, manage   bool
	}{
		{"owner bypass", "owner", false, false, true, true, true},
		{"admin bypass", "admin", false, false, true, true, true},
		{"member defaults", "member", true, false, false, true, false},
		{"member additions disabled", "member", false, false, false, false, false},
		{"explicit identity only", "member", false, true, true, false, false},
		{"removed grant", "", true, true, false, false, false},
		{"unknown role", "moderator", true, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := standalonePermissions(tc.role, tc.additions, tc.explicit)
			if got.CanPostAsChat != tc.post || got.CanAddMembers != tc.add || got.CanManagePermissions != tc.manage {
				t.Fatalf("permissions = %+v, want post/add/manage=%v/%v/%v", got, tc.post, tc.add, tc.manage)
			}
		})
	}
}
