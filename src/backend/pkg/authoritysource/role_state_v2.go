package authoritysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"sort"
	"time"
	"voice/backend/pkg/memberadmission"
)

const QualifiedSchemaVersion uint32 = 2

var ErrQualificationUnavailable = errors.New("qualified Role authority unavailable")
var ErrUnresolvedRoleEvidence = errors.New("unresolved legacy Role evidence")

type RoleStateV2 struct {
	SchemaVersion         uint32                `json:"schema_version"`
	SpaceID               string                `json:"space_id"`
	RoleRevision          uint64                `json:"role_revision"`
	SDKRevision           uint64                `json:"sdk_revision"`
	AllPermissions        uint64                `json:"all_permissions"`
	Retired               bool                  `json:"retired"`
	OwnershipFrozen       bool                  `json:"ownership_frozen"`
	DeletionState         string                `json:"deletion_state"`
	DeletionGeneration    uint64                `json:"deletion_generation"`
	Roles                 []RoleDefinitionV2    `json:"roles"`
	Owner                 *RoleOwnerLink        `json:"owner"`
	CommunityMemberRoleID string                `json:"community_member_role_id"`
	Contributions         []RoleContributionV2  `json:"contributions"`
	Floors                []RoleMembershipFloor `json:"floors"`
	Unresolved            []RoleAssignment      `json:"unresolved"`
	ChatOverrides         []RoleOverride        `json:"chat_overrides"`
	VoiceOverrides        []RoleOverride        `json:"voice_overrides"`
	SDKSessionGrants      []SDKSessionGrant     `json:"sdk_session_grants"`
}
type RoleDefinitionV2 struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Managed     bool   `json:"managed"`
	Position    int32  `json:"position"`
	Permissions uint64 `json:"permissions"`
	DefaultJoin bool   `json:"default_join"`
}
type RoleOwnerLink struct {
	ProfileID   string `json:"profile_id"`
	RoleID      string `json:"role_id"`
	Origin      string `json:"origin"`
	Generation  uint64 `json:"generation"`
	OperationID string `json:"operation_id"`
}
type RoleContributionV2 struct {
	ID              string                            `json:"id"`
	ProfileID       string                            `json:"profile_id"`
	RoleID          string                            `json:"role_id"`
	Origin          string                            `json:"origin"`
	Binding         memberadmission.MembershipBinding `json:"binding"`
	AuthorizationID string                            `json:"authorization_id"`
	Admission       *RoleAdmissionProof               `json:"admission"`
}
type RoleAdmissionProof struct {
	Binding       memberadmission.EnsureBinding `json:"binding"`
	ReceiptSHA256 []byte                        `json:"receipt_sha256"`
}
type RoleMembershipFloor struct {
	ProfileID      string `json:"profile_id"`
	Source         string `json:"source"`
	RetiredThrough uint64 `json:"retired_through"`
}

// Implemented by authenticated native observations and source2's independently
// authenticated/freshness-bounded Space adapter. Neither transport is replaced
// by accepting a client's JSON or treating a binding as grant authorization.
type MemberRoleObservation interface {
	SpaceID() string
	ProfileID() string
	Revision() uint64
	Available(time.Time) bool
	IsOwner(string, time.Time) bool
	Bindings(time.Time) ([]memberadmission.MembershipBinding, error)
	BindingEligible(memberadmission.MembershipBinding, time.Time) bool
	Eligible(memberadmission.MembershipBinding, []byte, time.Time) bool
}
type QualifiedRoleMember struct {
	Roles          []RoleDefinitionV2
	EffectiveMask  uint64
	EligibleMember bool
	CanonicalOwner bool
	TopPosition    int32
	Contributions  []RoleContributionV2
}

func QualifyRoleMember(state RoleStateV2, observation MemberRoleObservation, profile, chat, room string, now time.Time) (QualifiedRoleMember, error) {
	out := QualifiedRoleMember{Roles: []RoleDefinitionV2{}, Contributions: []RoleContributionV2{}, TopPosition: -1}
	if ValidateRoleStateV2(state) != nil || observation == nil || !canonicalID(profile) || state.SpaceID != observation.SpaceID() || profile != observation.ProfileID() || observation.Revision() == 0 || !observation.Available(now) || (chat != "" && !canonicalID(chat)) || (room != "" && !canonicalID(room)) || (chat != "" && room != "") {
		return out, ErrQualificationUnavailable
	}
	if state.Retired || state.OwnershipFrozen || (state.DeletionState != "" && state.DeletionState != "LIVE") {
		return out, ErrQualificationUnavailable
	}
	bindings, err := observation.Bindings(now)
	if err != nil {
		return out, ErrQualificationUnavailable
	}
	floors := map[string]uint64{}
	for _, f := range state.Floors {
		if f.ProfileID == profile {
			floors[f.Source] = f.RetiredThrough
		}
	}
	eligible := func(b memberadmission.MembershipBinding) bool {
		if memberadmission.ValidateMembershipBinding(b) != nil || !observation.BindingEligible(b, now) {
			return false
		}
		if b.Ordinary != nil {
			return (b.Ordinary.Legacy && floors["ORDINARY"] == 0) || b.Ordinary.MembershipGeneration > floors["ORDINARY"]
		}
		return b.Community.MembershipGeneration > floors["COMMUNITY"]
	}
	knownMember := false
	for _, b := range bindings {
		if eligible(b) {
			knownMember = true
		}
	}
	out.EligibleMember = knownMember
	selected := map[string]bool{}
	if state.Owner != nil && state.Owner.ProfileID == profile {
		if !observation.IsOwner(profile, now) {
			return out, ErrQualificationUnavailable
		}
		if !knownMember {
			return out, ErrQualificationUnavailable
		}
		selected[state.Owner.RoleID] = true
		out.CanonicalOwner = true
	}
	for _, c := range state.Contributions {
		if c.ProfileID != profile || !eligible(c.Binding) {
			continue
		}
		if c.Origin == "ORDINARY_DEFAULT" && !observation.Eligible(c.Binding, c.Admission.ReceiptSHA256, now) {
			continue
		}
		selected[c.RoleID] = true
		out.Contributions = append(out.Contributions, c)
	}
	// This is the fixed Role-owned managed Member policy, not today's default.
	// The independent community episode supplies its eligibility; legacy tuples
	// never manufacture or duplicate this typed baseline contribution.
	if state.CommunityMemberRoleID != "" {
		for _, b := range bindings {
			if b.Community != nil && eligible(b) {
				selected[state.CommunityMemberRoleID] = true
			}
		}
	}
	for _, legacy := range state.Unresolved {
		if legacy.ProfileID == profile && !selected[legacy.RoleID] {
			return out, ErrUnresolvedRoleEvidence
		}
	}
	for _, r := range state.Roles {
		if selected[r.ID] {
			out.Roles = append(out.Roles, r)
			out.EffectiveMask |= r.Permissions
			if r.Position > out.TopPosition {
				out.TopPosition = r.Position
			}
		}
	}
	if out.CanonicalOwner {
		out.EffectiveMask = state.AllPermissions
	} else {
		var allow, deny uint64
		overrides := state.ChatOverrides
		resource := chat
		if room != "" {
			overrides = state.VoiceOverrides
			resource = room
		}
		if resource != "" {
			for _, override := range overrides {
				if override.ResourceID == resource && selected[override.RoleID] {
					allow |= override.Allow
					deny |= override.Deny
				}
			}
		}
		out.EffectiveMask = (out.EffectiveMask | allow) &^ deny
	}
	if !observation.Available(now) {
		return QualifiedRoleMember{}, ErrQualificationUnavailable
	}
	return out, nil
}

func DecodeRoleStateV2(raw []byte) (RoleStateV2, error) {
	var state RoleStateV2
	if len(raw) == 0 || len(raw) > MaxStateBytes || json.Unmarshal(raw, &state) != nil {
		return state, ErrQualificationUnavailable
	}
	canonical, err := json.Marshal(state)
	if err != nil || !bytes.Equal(raw, canonical) || ValidateRoleStateV2(state) != nil {
		return RoleStateV2{}, ErrQualificationUnavailable
	}
	return state, nil
}

func ValidateRoleStateV2(state RoleStateV2) error {
	invalid := ErrQualificationUnavailable
	if state.SchemaVersion != QualifiedSchemaVersion || !canonicalID(state.SpaceID) || state.RoleRevision == 0 || state.SDKRevision == 0 || state.RoleRevision > math.MaxInt64 || state.SDKRevision > math.MaxInt64 || state.RoleRevision > math.MaxUint64-state.SDKRevision || state.AllPermissions == 0 || state.Roles == nil || state.Contributions == nil || state.Floors == nil || state.Unresolved == nil || state.ChatOverrides == nil || state.VoiceOverrides == nil || state.SDKSessionGrants == nil {
		return invalid
	}
	if len(state.Roles) > MaxSubjects || len(state.Contributions) > MaxSubjects || len(state.Floors) > MaxSubjects || len(state.Unresolved) > MaxSubjects {
		return invalid
	}
	base := RoleState{SchemaVersion: SchemaVersion, SpaceID: state.SpaceID, RoleRevision: state.RoleRevision, SDKRevision: state.SDKRevision, AllPermissions: state.AllPermissions, Retired: state.Retired, OwnershipFrozen: state.OwnershipFrozen, DeletionState: state.DeletionState, DeletionGeneration: state.DeletionGeneration, Roles: []RoleDefinition{}, Assignments: []RoleAssignment{}, ChatOverrides: state.ChatOverrides, VoiceOverrides: state.VoiceOverrides, SDKSessionGrants: state.SDKSessionGrants}
	roles := map[string]RoleDefinitionV2{}
	managed := map[string]bool{}
	memberPolicy := ""
	last := ""
	for _, r := range state.Roles {
		if r.Name == "" || r.Position < 0 || r.ID <= last {
			return invalid
		}
		last = r.ID
		if r.Managed {
			switch r.Name {
			case "Owner", "Admin", "Moderator", "Member", "Guest":
			default:
				return invalid
			}
			if managed[r.Name] {
				return invalid
			}
			managed[r.Name] = true
		}
		roles[r.ID] = r
		if r.Managed && r.Name == "Member" {
			memberPolicy = r.ID
		}
		base.Roles = append(base.Roles, RoleDefinition{ID: r.ID, Owner: r.Managed && r.Name == "Owner", Permissions: r.Permissions, DefaultJoin: r.DefaultJoin})
	}
	raw, err := json.Marshal(base)
	if err != nil {
		return invalid
	}
	if _, err = DecodeRoleState(raw); err != nil {
		return invalid
	}
	if state.Owner != nil {
		r, ok := roles[state.Owner.RoleID]
		if !ok || !r.Managed || r.Name != "Owner" || !canonicalID(state.Owner.ProfileID) || state.Owner.Generation == 0 || state.Owner.Generation > math.MaxInt64 {
			return invalid
		}
		switch state.Owner.Origin {
		case "BOOTSTRAP":
			if state.Owner.OperationID != "" {
				return invalid
			}
		case "TRANSFER":
			if !canonicalID(state.Owner.OperationID) {
				return invalid
			}
		default:
			return invalid
		}
	}
	if state.CommunityMemberRoleID != "" {
		r, ok := roles[state.CommunityMemberRoleID]
		if !ok || !r.Managed || r.Name != "Member" {
			return invalid
		}
	}
	if state.CommunityMemberRoleID != memberPolicy {
		return invalid
	}
	last = ""
	for _, c := range state.Contributions {
		r, ok := roles[c.RoleID]
		key := c.Origin + "/" + c.ID
		if !ok || !canonicalID(c.ID) || !canonicalID(c.ProfileID) || !canonicalID(c.AuthorizationID) || key <= last || memberadmission.ValidateMembershipBinding(c.Binding) != nil || (r.Managed && r.Name == "Owner") {
			return invalid
		}
		last = key
		switch c.Origin {
		case "ORDINARY_DEFAULT":
			if c.Admission == nil || c.Binding.Ordinary == nil || c.Binding.Ordinary.Legacy {
				return invalid
			}
			b := c.Admission.Binding
			if memberadmission.ValidateEnsure(b) != nil || b.SpaceID != state.SpaceID || b.ProfileID != c.ProfileID || b.AdmissionOperationID != c.ID || c.AuthorizationID != c.ID || b.AdmissionOperationID != c.Binding.Ordinary.PublicationIdentity || b.MembershipGeneration != c.Binding.Ordinary.MembershipGeneration {
				return invalid
			}
			digest, err := memberadmission.EnsureReceiptFingerprint(b, c.RoleID)
			if err != nil || !bytes.Equal(digest[:], c.Admission.ReceiptSHA256) {
				return invalid
			}
		case "INTERACTIVE":
			if c.Admission != nil {
				return invalid
			}
		case "COMMUNITY_BASELINE":
			if c.Admission != nil || c.Binding.Community == nil || !r.Managed || r.Name != "Member" {
				return invalid
			}
		default:
			return invalid
		}
	}
	last = ""
	for _, floor := range state.Floors {
		key := floor.ProfileID + "/" + floor.Source
		if !canonicalID(floor.ProfileID) || key <= last || (floor.Source != "ORDINARY" && floor.Source != "COMMUNITY") || floor.RetiredThrough == 0 || floor.RetiredThrough > math.MaxInt64 {
			return invalid
		}
		last = key
	}
	last = ""
	for _, legacy := range state.Unresolved {
		key := legacy.ProfileID + "/" + legacy.RoleID
		if !canonicalID(legacy.ProfileID) || key <= last {
			return invalid
		}
		if _, ok := roles[legacy.RoleID]; !ok {
			return invalid
		}
		if state.Owner != nil && legacy == (RoleAssignment{ProfileID: state.Owner.ProfileID, RoleID: state.Owner.RoleID}) {
			return invalid
		}
		last = key
	}
	return nil
}

func SortRoleStateV2(state *RoleStateV2) {
	sort.Slice(state.Roles, func(i, j int) bool { return state.Roles[i].ID < state.Roles[j].ID })
	sort.Slice(state.Contributions, func(i, j int) bool {
		a, b := state.Contributions[i], state.Contributions[j]
		return a.Origin+"/"+a.ID < b.Origin+"/"+b.ID
	})
	sort.Slice(state.Floors, func(i, j int) bool {
		a, b := state.Floors[i], state.Floors[j]
		return a.ProfileID+"/"+a.Source < b.ProfileID+"/"+b.Source
	})
	sort.Slice(state.Unresolved, func(i, j int) bool {
		a, b := state.Unresolved[i], state.Unresolved[j]
		return a.ProfileID+"/"+a.RoleID < b.ProfileID+"/"+b.RoleID
	})
}
