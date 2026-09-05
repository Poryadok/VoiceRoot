package userevents

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPresenceChangedEvent_UsesCanonicalDeltaAndLegacyCurrentStatus(t *testing.T) {
	tests := []struct {
		name      string
		oldStatus string
		newStatus string
	}{
		{
			name:      "first observation",
			oldStatus: "",
			newStatus: "online",
		},
		{
			name:      "enum transition",
			oldStatus: "online",
			newStatus: "dnd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := newPresenceChangedEvent("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", tt.oldStatus, tt.newStatus)
			change := event.GetPresenceChange()
			require.NotNil(t, change)
			require.Equal(t, "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", change.GetProfileId())
			require.Equal(t, tt.oldStatus, change.GetOldStatus())
			require.Equal(t, tt.newStatus, change.GetNewStatus())
			require.Equal(t, tt.newStatus, change.GetStatus(), "legacy status must remain the current/new status")
		})
	}
}
