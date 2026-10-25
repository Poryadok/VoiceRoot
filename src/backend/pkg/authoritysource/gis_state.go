package authoritysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"slices"
)

// GISState contains only GIS-owned authorization facts. SpaceID binds the
// publisher's request; GIS cannot attest cross-store Space/resource ownership.
// Subjects is the complete requested set: no matching binding means missing,
// not an implicit regular account, profile, or grant.
type GISState struct {
	SchemaVersion       uint32            `json:"schema_version"`
	Revision            uint64            `json:"revision"`
	SpaceID             string            `json:"space_id"`
	EnvironmentID       string            `json:"environment_id"`
	EnvironmentExists   bool              `json:"environment_exists"`
	ApplicationID       string            `json:"application_id"`
	ApplicationStatus   string            `json:"application_status"`
	ApplicationRevision uint64            `json:"application_revision"`
	EnvironmentKind     string            `json:"environment_kind"`
	EnvironmentStatus   string            `json:"environment_status"`
	EnvironmentRevision uint64            `json:"environment_revision"`
	Providers           []string          `json:"providers"`
	PlayerScopes        []string          `json:"player_scopes"`
	RedirectURIs        []string          `json:"redirect_uris"`
	AllowedOrigins      []string          `json:"allowed_origins"`
	Subjects            []string          `json:"subjects"`
	Bindings            []GISBinding      `json:"bindings"`
	Installations       []GISInstallation `json:"installations"`
	Mappings            []GISMapping      `json:"mappings"`
	ManagedChats        []GISManagedChat  `json:"managed_chats"`
	Sessions            []GISSession      `json:"sessions"`
}
type GISBinding struct {
	BindingID         string `json:"binding_id"`
	AccountID         string `json:"account_id"`
	ActorID           string `json:"actor_id"`
	ProfileID         string `json:"profile_id"`
	DeviceID          string `json:"device_id"`
	Status            string `json:"status"`
	AuthorityRevision uint64 `json:"authority_revision"`
}
type GISInstallation struct {
	InstallationID string `json:"installation_id"`
	BotID          string `json:"bot_id"`
	Status         string `json:"status"`
}
type GISMapping struct {
	MappingID  string `json:"mapping_id"`
	Kind       string `json:"kind"`
	ResourceID string `json:"resource_id"`
	ChatID     string `json:"chat_id"`
	Status     string `json:"status"`
	Revision   uint64 `json:"revision"`
}
type GISManagedChat struct {
	BindingID            string `json:"binding_id"`
	ChatID               string `json:"chat_id"`
	Status               string `json:"status"`
	RosterRevision       uint64 `json:"roster_revision"`
	LeaseUntilUnixMillis int64  `json:"lease_until_unix_millis"`
}
type GISSession struct {
	SessionID      string   `json:"session_id"`
	Kind           string   `json:"kind"`
	Status         string   `json:"status"`
	RosterRevision uint64   `json:"roster_revision"`
	RosterComplete bool     `json:"roster_complete"`
	Members        []string `json:"members"`
	ChatID         string   `json:"chat_id"`
	VoiceRoomID    string   `json:"voice_room_id"`
}

func DecodeGISState(raw []byte) (GISState, error) {
	var s GISState
	bad := errors.New("invalid canonical GIS authority state")
	if len(raw) > MaxStateBytes || json.Unmarshal(raw, &s) != nil {
		return GISState{}, bad
	}
	canonical, err := json.Marshal(s)
	if err != nil || !bytes.Equal(raw, canonical) || s.SchemaVersion != SchemaVersion || !gisRevision(s.Revision) || !canonicalID(s.SpaceID) || !canonicalID(s.EnvironmentID) || s.Subjects == nil || !validIDs(s.Subjects) || s.Bindings == nil || s.Installations == nil || s.Mappings == nil || s.ManagedChats == nil || s.Sessions == nil {
		return GISState{}, bad
	}
	for _, n := range []int{len(s.Bindings), len(s.Installations), len(s.Mappings), len(s.ManagedChats), len(s.Sessions)} {
		if n > MaxSubjects {
			return GISState{}, bad
		}
	}
	if !s.EnvironmentExists {
		if s.ApplicationID != "" || s.ApplicationStatus != "" || s.ApplicationRevision != 0 || s.EnvironmentKind != "" || s.EnvironmentStatus != "" || s.EnvironmentRevision != 0 || len(s.Providers)+len(s.PlayerScopes)+len(s.RedirectURIs)+len(s.AllowedOrigins)+len(s.Bindings)+len(s.Installations)+len(s.Mappings)+len(s.ManagedChats)+len(s.Sessions) != 0 {
			return GISState{}, bad
		}
		return s, nil
	}
	if !canonicalID(s.ApplicationID) || !gisRevision(s.ApplicationRevision) || !gisRevision(s.EnvironmentRevision) || !slices.Contains([]string{"draft", "sandbox", "active", "suspended", "retired"}, s.ApplicationStatus) || !slices.Contains([]string{"pending", "active", "suspended", "retired"}, s.EnvironmentStatus) || !slices.Contains([]string{"sandbox", "production"}, s.EnvironmentKind) {
		return GISState{}, bad
	}
	for _, items := range [][]string{s.Providers, s.PlayerScopes, s.RedirectURIs, s.AllowedOrigins} {
		if items == nil || len(items) > MaxSubjects || !slices.IsSorted(items) {
			return GISState{}, bad
		}
		for i, v := range items {
			if v == "" || i > 0 && v == items[i-1] {
				return GISState{}, bad
			}
		}
	}
	for _, v := range s.Providers {
		if v != "google" {
			return GISState{}, bad
		}
	}
	for _, v := range s.PlayerScopes {
		if !slices.Contains([]string{"game.chat.read", "game.chat.send", "game.identity.read", "game.invites.create", "game.presence.write", "game.voice.join"}, v) {
			return GISState{}, bad
		}
	}
	last := ""
	bindings := map[string]bool{}
	for _, b := range s.Bindings {
		if !canonicalID(b.BindingID) || b.BindingID <= last || !canonicalID(b.AccountID) || !canonicalID(b.ActorID) || !canonicalID(b.ProfileID) || !canonicalID(b.DeviceID) || !gisRevision(b.AuthorityRevision) || !slices.Contains([]string{"pending", "active", "revoking", "revoked"}, b.Status) || (!slices.Contains(s.Subjects, b.ActorID) && !slices.Contains(s.Subjects, b.ProfileID)) {
			return GISState{}, bad
		}
		last = b.BindingID
		bindings[b.BindingID] = true
	}
	last = ""
	for _, i := range s.Installations {
		if !canonicalID(i.InstallationID) || i.InstallationID <= last || i.BotID != "" && !canonicalID(i.BotID) || !slices.Contains([]string{"active", "suspended", "revoked"}, i.Status) {
			return GISState{}, bad
		}
		last = i.InstallationID
	}
	last = ""
	for _, m := range s.Mappings {
		if !canonicalID(m.MappingID) || m.MappingID <= last || !canonicalID(m.ResourceID) || !gisRevision(m.Revision) || !slices.Contains([]string{"chat", "voice", "space"}, m.Kind) || !slices.Contains([]string{"active", "retired", "tombstoned"}, m.Status) || (m.Kind == "space") != (m.ChatID == "") || m.ChatID != "" && !canonicalID(m.ChatID) || m.Kind == "chat" && m.ChatID != m.ResourceID {
			return GISState{}, bad
		}
		last = m.MappingID
	}
	last = ""
	for _, m := range s.ManagedChats {
		key := m.BindingID + "/" + m.ChatID
		if !bindings[m.BindingID] || !canonicalID(m.ChatID) || key <= last || !gisRevision(m.RosterRevision) || m.LeaseUntilUnixMillis <= 0 || !slices.Contains([]string{"active", "revoked"}, m.Status) {
			return GISState{}, bad
		}
		last = key
	}
	last = ""
	for _, v := range s.Sessions {
		if !canonicalID(v.SessionID) || v.SessionID <= last || !gisRevision(v.RosterRevision) || !slices.Contains([]string{"party", "match", "fleet"}, v.Kind) || !slices.Contains([]string{"provisioning", "active", "closing", "closed", "failed"}, v.Status) || v.Members == nil || !validIDs(v.Members) || v.ChatID != "" && !canonicalID(v.ChatID) || v.VoiceRoomID != "" && !canonicalID(v.VoiceRoomID) {
			return GISState{}, bad
		}
		for _, p := range v.Members {
			if !slices.Contains(s.Subjects, p) {
				return GISState{}, bad
			}
		}
		last = v.SessionID
	}
	return s, nil
}
func gisRevision(v uint64) bool { return v > 0 && v <= math.MaxInt64 }

// Time never rewrites canonical bytes at a revision. The publisher evaluates
// every raw lease and cannot renew active rights beyond this earlier cutoff.
func (s GISState) ValidUntilUnixMillis(now int64) int64 {
	if !s.EnvironmentExists || s.EnvironmentStatus != "active" || s.ApplicationStatus != "sandbox" && s.ApplicationStatus != "active" {
		return 0
	}
	active := map[string]bool{}
	for _, b := range s.Bindings {
		active[b.BindingID] = b.Status == "active"
	}
	var until int64
	for _, m := range s.ManagedChats {
		if active[m.BindingID] && m.Status == "active" && m.LeaseUntilUnixMillis > now && (until == 0 || m.LeaseUntilUnixMillis < until) {
			until = m.LeaseUntilUnixMillis
		}
	}
	return until
}
