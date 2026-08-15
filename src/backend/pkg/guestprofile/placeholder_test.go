package guestprofile

import "testing"

func TestIsPlaceholderDisplayName(t *testing.T) {
	t.Parallel()
	accountID := "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
	tests := []struct {
		display string
		want    bool
	}{
		{display: accountID, want: true},
		{display: "A1B2C3D4E5F67890ABCDEF1234567890", want: true},
		{display: "Guest Nickname", want: false},
	}
	for _, tc := range tests {
		if got := IsPlaceholderDisplayName(accountID, tc.display); got != tc.want {
			t.Fatalf("display %q: got %v want %v", tc.display, got, tc.want)
		}
	}
}
