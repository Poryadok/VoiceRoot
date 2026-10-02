package authoritysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"slices"
)

type SpaceState struct {
	SchemaVersion      uint32              `json:"schema_version"`
	SpaceID            string              `json:"space_id"`
	Revision           uint64              `json:"revision"`
	Exists             bool                `json:"exists"`
	OwnerProfileID     string              `json:"owner_profile_id"`
	Visibility         string              `json:"visibility"`
	AllowGuests        bool                `json:"allow_guests"`
	OwnershipFrozen    bool                `json:"ownership_frozen"`
	DeletionPhase      string              `json:"deletion_phase"`
	DeletionGeneration uint64              `json:"deletion_generation"`
	Purged             bool                `json:"purged"`
	Members            []string            `json:"members"`
	BannedAccounts     []string            `json:"banned_accounts"`
	Timeouts           []SpaceTimeout      `json:"timeouts"`
	VoiceRooms         []string            `json:"voice_rooms"`
	Categories         []string            `json:"categories"`
	Tree               []SpaceTreeResource `json:"tree"`
	Community          *CommunityAuthority `json:"community"`
	CommunityMembers   []CommunityMember   `json:"community_members"`
}

type SpaceTimeout struct {
	ProfileID       string `json:"profile_id"`
	UntilUnixMillis int64  `json:"until_unix_millis"`
}

type SpaceTreeResource struct {
	NodeID     string `json:"node_id"`
	CategoryID string `json:"category_id"`
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
}

type CommunityAuthority struct {
	ApplicationID        string `json:"application_id"`
	EnvironmentID        string `json:"environment_id"`
	OwnerAccountID       string `json:"owner_account_id"`
	OwnerProfileID       string `json:"owner_profile_id"`
	OwnerGeneration      uint64 `json:"owner_generation"`
	Status               string `json:"status"`
	RosterRevision       uint64 `json:"roster_revision"`
	LeaseUntilUnixMillis int64  `json:"lease_until_unix_millis"`
}

type CommunityMember struct {
	ProfileID            string `json:"profile_id"`
	OwnerGeneration      uint64 `json:"owner_generation"`
	SourceRevision       uint64 `json:"source_revision"`
	LeaseUntilUnixMillis int64  `json:"lease_until_unix_millis"`
	Revoked              bool   `json:"revoked"`
}

func DecodeSpaceState(raw []byte) (SpaceState, error) {
	var state SpaceState
	if len(raw) == 0 || len(raw) > MaxStateBytes || json.Unmarshal(raw, &state) != nil {
		return SpaceState{}, errors.New("invalid Space state")
	}
	canonical, err := json.Marshal(state)
	if err != nil || !bytes.Equal(canonical, raw) || state.SchemaVersion != SchemaVersion || !canonicalID(state.SpaceID) || state.Revision == 0 || state.Revision > math.MaxInt64 {
		return SpaceState{}, errors.New("invalid canonical Space state")
	}
	if state.Members == nil || state.BannedAccounts == nil || state.VoiceRooms == nil || state.Categories == nil || state.Timeouts == nil || state.Tree == nil || state.CommunityMembers == nil || !validIDs(state.Members) || !validIDs(state.BannedAccounts) || !validIDs(state.VoiceRooms) || !validIDs(state.Categories) || len(state.Timeouts) > MaxSubjects || len(state.Tree) > MaxSubjects || len(state.CommunityMembers) > MaxSubjects {
		return SpaceState{}, errors.New("incomplete Space sets")
	}
	if state.Exists {
		if !canonicalID(state.OwnerProfileID) || (state.Visibility != "private" && state.Visibility != "public" && state.Visibility != "invite_only") || state.Purged {
			return SpaceState{}, errors.New("invalid live Space identity")
		}
	} else if state.OwnerProfileID != "" || state.Visibility != "" || state.AllowGuests || len(state.Members)+len(state.BannedAccounts)+len(state.Timeouts)+len(state.VoiceRooms)+len(state.Categories)+len(state.Tree)+len(state.CommunityMembers) > 0 || state.Community != nil {
		return SpaceState{}, errors.New("missing Space retains active state")
	}
	switch state.DeletionPhase {
	case "":
		if state.DeletionGeneration != 0 {
			return SpaceState{}, errors.New("invalid empty lifecycle")
		}
	case "LIVE", "SCHEDULE_PENDING", "FREEZE_PENDING", "SCHEDULED", "RESTORE_DECIDED", "PURGE_DECIDED", "PURGING", "PURGED":
		if state.DeletionGeneration == 0 || state.DeletionGeneration > math.MaxInt64 {
			return SpaceState{}, errors.New("invalid lifecycle generation")
		}
	default:
		return SpaceState{}, errors.New("unknown Space lifecycle")
	}
	for i, item := range state.Timeouts {
		if !canonicalID(item.ProfileID) || item.UntilUnixMillis <= 0 || (i > 0 && state.Timeouts[i-1].ProfileID >= item.ProfileID) {
			return SpaceState{}, errors.New("invalid Space timeouts")
		}
	}
	resources := map[string]bool{}
	categories, rooms := map[string]bool{}, map[string]bool{}
	for _, id := range state.Categories {
		categories[id] = true
	}
	for _, id := range state.VoiceRooms {
		rooms[id] = true
	}
	for i, item := range state.Tree {
		if !canonicalID(item.NodeID) || !canonicalID(item.ResourceID) || (item.CategoryID != "" && !categories[item.CategoryID]) || (i > 0 && state.Tree[i-1].NodeID >= item.NodeID) || resources[item.Kind+item.ResourceID] {
			return SpaceState{}, errors.New("invalid Space tree")
		}
		if item.Kind != "text_chat" && item.Kind != "voice_room" || item.Kind == "voice_room" && !rooms[item.ResourceID] {
			return SpaceState{}, errors.New("Space tree scope mismatch")
		}
		resources[item.Kind+item.ResourceID] = true
	}
	if c := state.Community; c != nil {
		if !canonicalID(c.ApplicationID) || !canonicalID(c.EnvironmentID) || !canonicalID(c.OwnerAccountID) || !canonicalID(c.OwnerProfileID) || c.OwnerProfileID != state.OwnerProfileID || c.OwnerGeneration == 0 || c.OwnerGeneration > math.MaxInt64 || c.RosterRevision > math.MaxInt64 || (c.Status != "active" && c.Status != "recovery_pending") || c.LeaseUntilUnixMillis < 0 || (c.RosterRevision == 0) != (c.LeaseUntilUnixMillis == 0) {
			return SpaceState{}, errors.New("invalid community authority")
		}
	}
	for i, item := range state.CommunityMembers {
		if state.Community == nil || !canonicalID(item.ProfileID) || item.OwnerGeneration == 0 || item.OwnerGeneration > math.MaxInt64 || item.SourceRevision == 0 || item.SourceRevision > math.MaxInt64 || item.LeaseUntilUnixMillis <= 0 || (i > 0 && state.CommunityMembers[i-1].ProfileID >= item.ProfileID) {
			return SpaceState{}, errors.New("invalid community members")
		}
		if !item.Revoked && item.OwnerGeneration == state.Community.OwnerGeneration && item.SourceRevision != state.Community.RosterRevision {
			return SpaceState{}, errors.New("community member revision conflicts with owner")
		}
	}
	return state, nil
}

// MemberAt follows the owning Space membership union. Public visibility and
// allow_guests are admission settings; neither creates an existing membership.
func (s SpaceState) MemberAt(profile string, nowUnixMillis int64) bool {
	if !s.Exists || s.Purged || s.OwnershipFrozen || s.DeletionPhase != "" && s.DeletionPhase != "LIVE" || !canonicalID(profile) || nowUnixMillis <= 0 {
		return false
	}
	if slices.Contains(s.Members, profile) {
		return true
	}
	c := s.Community
	if c == nil || c.Status != "active" || c.LeaseUntilUnixMillis <= nowUnixMillis {
		return false
	}
	for _, member := range s.CommunityMembers {
		if member.ProfileID == profile && !member.Revoked && member.OwnerGeneration == c.OwnerGeneration && member.LeaseUntilUnixMillis > nowUnixMillis {
			return true
		}
	}
	return false
}

// ValidUntil bounds time-derived grants even if no write changes the revision.
// Raw canonical state is stable at a revision; expired rows are retained, not
// silently filtered out of an allegedly complete snapshot.
func (s SpaceState) ValidUntil(nowUnixMillis int64) int64 {
	var earliest int64
	c := s.Community
	if c == nil || c.Status != "active" || c.LeaseUntilUnixMillis <= nowUnixMillis {
		return 0
	}
	for _, member := range s.CommunityMembers {
		if member.Revoked || member.OwnerGeneration != c.OwnerGeneration || member.LeaseUntilUnixMillis <= nowUnixMillis {
			continue
		}
		until := min(member.LeaseUntilUnixMillis, c.LeaseUntilUnixMillis)
		if earliest == 0 || until < earliest {
			earliest = until
		}
	}
	return earliest
}
