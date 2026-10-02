package authoritysource

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRoleStateDecoderRefusesUnknownMissingDuplicateAndMalformedAuthority(t *testing.T) {
	role, profile, room := uuid.NewString(), uuid.NewString(), uuid.NewString()
	state := RoleState{SchemaVersion: 1, SpaceID: uuid.NewString(), RoleRevision: 2, SDKRevision: 3, AllPermissions: 7,
		Roles: []RoleDefinition{{ID: role, Permissions: 1, DefaultJoin: true}}, Assignments: []RoleAssignment{{ProfileID: profile, RoleID: role}},
		ChatOverrides: []RoleOverride{}, VoiceOverrides: []RoleOverride{{ResourceID: room, RoleID: role, Deny: 1}}, SDKSessionGrants: []SDKSessionGrant{}}
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	_, err = DecodeRoleState(raw)
	require.NoError(t, err)
	for name, mutation := range map[string]func(*RoleState){
		"unknown version":     func(s *RoleState) { s.SchemaVersion = 2 },
		"missing clock":       func(s *RoleState) { s.SDKRevision = 0 },
		"partial roles":       func(s *RoleState) { s.Roles = nil },
		"unknown role":        func(s *RoleState) { s.Assignments[0].RoleID = uuid.NewString() },
		"unknown permission":  func(s *RoleState) { s.Roles[0].Permissions = 8 },
		"zero profile":        func(s *RoleState) { s.Assignments[0].ProfileID = uuid.Nil.String() },
		"contradictory fence": func(s *RoleState) { s.DeletionGeneration = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			var changed RoleState
			require.NoError(t, json.Unmarshal(raw, &changed))
			mutation(&changed)
			encoded, err := json.Marshal(changed)
			require.NoError(t, err)
			_, err = DecodeRoleState(encoded)
			require.Error(t, err)
		})
	}
	for _, extra := range []string{`,"future_authority":true}`, `,"sdk_revision":3}`} {
		changed := append(append([]byte{}, raw[:len(raw)-1]...), []byte(extra)...)
		_, err = DecodeRoleState(changed)
		require.Error(t, err)
	}
}

func TestRoleStateFrozenAndUnassignedNeverReceiveDefaultPermissions(t *testing.T) {
	profile, role, room := uuid.NewString(), uuid.NewString(), uuid.NewString()
	state := RoleState{AllPermissions: 7, Roles: []RoleDefinition{{ID: role, Permissions: 7, DefaultJoin: true}}, Assignments: []RoleAssignment{}}
	require.Zero(t, state.EffectiveMask(profile, room, true))
	state.Assignments = []RoleAssignment{{ProfileID: profile, RoleID: role}}
	require.Equal(t, uint64(7), state.EffectiveMask(profile, room, true))
	for _, frozen := range []string{"FROZEN", "PURGE_DECIDED"} {
		state.DeletionState = frozen
		require.Zero(t, state.EffectiveMask(profile, room, true))
	}
	state.DeletionState = "LIVE"
	state.OwnershipFrozen = true
	require.Zero(t, state.EffectiveMask(profile, room, true))
	state.OwnershipFrozen = false
	state.Retired = true
	require.Zero(t, state.EffectiveMask(profile, room, true))
}
