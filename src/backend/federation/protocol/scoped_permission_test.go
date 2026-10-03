package protocol

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSnapshotValidatesScopedPermissionWithoutInventingMediaAuthority(t *testing.T) {
	now := time.Now()
	permission := Permission{AccountID: uuid.NewString(), ProfileID: uuid.NewString(), ResourceID: uuid.NewString(), SessionEpoch: 1, Actions: []string{"media"}, RoomName: "canonical-room", RoutingGeneration: 2,
		ApplicationID: uuid.NewString(), EnvironmentID: uuid.NewString(), BindingID: uuid.NewString(), InstallationID: uuid.NewString()}
	validate := func(p Permission) error {
		return (Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: 1, ValidUntil: now.Add(time.Second).UnixMilli(), Permissions: []Permission{p}}).Validate(now)
	}
	require.NoError(t, validate(permission))
	for name, change := range map[string]func(*Permission){
		"partial application":        func(p *Permission) { p.EnvironmentID = "" },
		"noncanonical binding":       func(p *Permission) { p.BindingID = "binding" },
		"orphan installation":        func(p *Permission) { p.ApplicationID, p.EnvironmentID, p.BindingID = "", "", "" },
		"negative route":             func(p *Permission) { p.RoutingGeneration = -1 },
		"room without route":         func(p *Permission) { p.RoutingGeneration = 0 },
		"routed media without room":  func(p *Permission) { p.RoomName = "" },
		"room control character":     func(p *Permission) { p.RoomName = "room\nname" },
		"room on content permission": func(p *Permission) { p.Actions = []string{"read"} },
	} {
		t.Run(name, func(t *testing.T) {
			changed := permission
			change(&changed)
			require.ErrorIs(t, validate(changed), ErrInvalid)
		})
	}
	content := permission
	content.RoomName = ""
	content.Actions = []string{"read", "write", "subscribe"}
	require.NoError(t, validate(content), "content route carries generation without an RTC room")
	legacy := permission
	legacy.RoomName, legacy.RoutingGeneration = "", 0
	require.NoError(t, validate(legacy), "legacy transport shape remains decodable; scoped media consumer separately denies it")
}
