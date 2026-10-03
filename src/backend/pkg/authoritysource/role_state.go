package authoritysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
)

// RoleState carries authorization only. Receipt/proof bytes and display fields
// stay in Role. Empty collections are explicit [] and never omitted or null.
type RoleState struct {
	SchemaVersion      uint32            `json:"schema_version"`
	SpaceID            string            `json:"space_id"`
	RoleRevision       uint64            `json:"role_revision"`
	SDKRevision        uint64            `json:"sdk_revision"`
	AllPermissions     uint64            `json:"all_permissions"`
	Retired            bool              `json:"retired"`
	OwnershipFrozen    bool              `json:"ownership_frozen"`
	DeletionState      string            `json:"deletion_state"`
	DeletionGeneration uint64            `json:"deletion_generation"`
	Roles              []RoleDefinition  `json:"roles"`
	Assignments        []RoleAssignment  `json:"assignments"`
	ChatOverrides      []RoleOverride    `json:"chat_overrides"`
	VoiceOverrides     []RoleOverride    `json:"voice_overrides"`
	SDKSessionGrants   []SDKSessionGrant `json:"sdk_session_grants"`
}
type RoleDefinition struct {
	ID          string `json:"id"`
	Owner       bool   `json:"owner"`
	Permissions uint64 `json:"permissions"`
	DefaultJoin bool   `json:"default_join"`
}
type RoleAssignment struct {
	ProfileID string `json:"profile_id"`
	RoleID    string `json:"role_id"`
}
type RoleOverride struct {
	ResourceID string `json:"resource_id"`
	RoleID     string `json:"role_id"`
	Allow      uint64 `json:"allow"`
	Deny       uint64 `json:"deny"`
}
type SDKSessionGrant struct {
	ApplicationID  string `json:"application_id"`
	EnvironmentID  string `json:"environment_id"`
	SessionID      string `json:"session_id"`
	VoiceRoomID    string `json:"voice_room_id"`
	ProfileID      string `json:"profile_id"`
	RosterRevision uint64 `json:"roster_revision"`
}

// DecodeRoleState also requires the exact fixed encoding emitted by the owner.
// Re-encoding rejects missing/unknown/duplicate fields and alternate encodings.
func DecodeRoleState(raw []byte) (RoleState, error) {
	var state RoleState
	if len(raw) > MaxStateBytes || json.Unmarshal(raw, &state) != nil {
		return state, errors.New("invalid Role source state")
	}
	canonical, err := json.Marshal(state)
	if err != nil || !bytes.Equal(raw, canonical) || state.SchemaVersion != SchemaVersion || !canonicalID(state.SpaceID) || state.RoleRevision == 0 || state.SDKRevision == 0 || state.AllPermissions == 0 || state.Roles == nil || state.Assignments == nil || state.ChatOverrides == nil || state.VoiceOverrides == nil || state.SDKSessionGrants == nil {
		return RoleState{}, errors.New("noncanonical Role source state")
	}
	if state.RoleRevision > math.MaxInt64 || state.SDKRevision > math.MaxInt64 || len(state.Roles) > MaxSubjects || len(state.Assignments) > MaxSubjects || len(state.ChatOverrides) > MaxSubjects || len(state.VoiceOverrides) > MaxSubjects || len(state.SDKSessionGrants) > MaxSubjects {
		return RoleState{}, errors.New("role source bound exceeded")
	}
	if (state.DeletionState == "" && state.DeletionGeneration != 0) || (state.DeletionState != "" && (state.DeletionGeneration == 0 || (state.DeletionState != "LIVE" && state.DeletionState != "FROZEN" && state.DeletionState != "PURGE_DECIDED"))) {
		return RoleState{}, errors.New("invalid Role deletion state")
	}
	roles := map[string]bool{}
	last := ""
	defaults := 0
	for _, item := range state.Roles {
		if !canonicalID(item.ID) || item.ID <= last || item.Permissions&^state.AllPermissions != 0 {
			return RoleState{}, errors.New("invalid Role definition")
		}
		last = item.ID
		roles[item.ID] = true
		if item.DefaultJoin {
			defaults++
		}
	}
	if defaults > 1 {
		return RoleState{}, errors.New("multiple default Role definitions")
	}
	last = ""
	for _, item := range state.Assignments {
		key := item.ProfileID + "/" + item.RoleID
		if !canonicalID(item.ProfileID) || !roles[item.RoleID] || key <= last {
			return RoleState{}, errors.New("invalid Role assignment")
		}
		last = key
	}
	for _, items := range [][]RoleOverride{state.ChatOverrides, state.VoiceOverrides} {
		last = ""
		for _, item := range items {
			key := item.ResourceID + "/" + item.RoleID
			if !canonicalID(item.ResourceID) || !roles[item.RoleID] || key <= last || (item.Allow|item.Deny)&^state.AllPermissions != 0 {
				return RoleState{}, errors.New("invalid Role override")
			}
			last = key
		}
	}
	last = ""
	for _, item := range state.SDKSessionGrants {
		key := item.ApplicationID + "/" + item.EnvironmentID + "/" + item.SessionID + "/" + item.ProfileID
		if !canonicalID(item.ApplicationID) || !canonicalID(item.EnvironmentID) || !canonicalID(item.SessionID) || !canonicalID(item.VoiceRoomID) || !canonicalID(item.ProfileID) || key <= last || item.RosterRevision == 0 || item.RosterRevision > math.MaxInt64 {
			return RoleState{}, errors.New("invalid SDK Role grant")
		}
		last = key
	}
	return state, nil
}

// EffectiveMask follows Role's canonical fold: no assignment means no rights;
// Owner bypasses overrides; other assigned roles combine with default join,
// then only the assigned roles' resource allows/denies are applied.
func (state RoleState) EffectiveMask(profileID, resourceID string, voice bool) uint64 {
	if state.Retired || state.OwnershipFrozen || (state.DeletionState != "" && state.DeletionState != "LIVE") {
		return 0
	}
	assigned := map[string]bool{}
	for _, item := range state.Assignments {
		if item.ProfileID == profileID {
			assigned[item.RoleID] = true
		}
	}
	if len(assigned) == 0 {
		return 0
	}
	var mask uint64
	for _, role := range state.Roles {
		if assigned[role.ID] && role.Owner {
			return state.AllPermissions
		}
		if assigned[role.ID] || role.DefaultJoin {
			mask |= role.Permissions
		}
	}
	items := state.ChatOverrides
	if voice {
		items = state.VoiceOverrides
	}
	var allow, deny uint64
	for _, item := range items {
		if item.ResourceID == resourceID && assigned[item.RoleID] {
			allow |= item.Allow
			deny |= item.Deny
		}
	}
	return (mask | allow) &^ deny
}
