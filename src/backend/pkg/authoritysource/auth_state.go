package authoritysource

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"regexp"
)

// AuthState is the canonical Java Auth projection. SDK session rows are raw
// device prerequisites, never proof of a particular caller's bearer token.
// Game bindings and message grants do not establish Voice room permissions.
type AuthState struct {
	SchemaVersion    uint32                `json:"schema_version"`
	Revision         uint64                `json:"revision"`
	Accounts         []AuthAccount         `json:"accounts"`
	SDKIdentities    []AuthSDKIdentity     `json:"sdk_identities"`
	SDKDevices       []AuthSDKDevice       `json:"sdk_devices"`
	SDKKeys          []AuthSDKKey          `json:"sdk_keys"`
	SDKSessions      []AuthSDKSession      `json:"sdk_sessions"`
	SDKBindings      []AuthSDKBinding      `json:"sdk_bindings"`
	SDKConversions   []AuthSDKConversion   `json:"sdk_conversions"`
	SDKMessageGrants []AuthSDKMessageGrant `json:"sdk_message_grants"`
}
type AuthAccount struct {
	AccountID        string `json:"account_id"`
	Exists           bool   `json:"exists"`
	Type             string `json:"type"`
	Status           string `json:"status"`
	Deleted          bool   `json:"deleted"`
	EmailPending     bool   `json:"email_pending"`
	SessionEpoch     uint64 `json:"session_epoch"`
	SecurityRevision uint64 `json:"security_revision"`
}
type AuthSDKIdentity struct {
	AccountID           string `json:"account_id"`
	Exists              bool   `json:"exists"`
	ActorID             string `json:"actor_id"`
	ApplicationID       string `json:"application_id"`
	EnvironmentID       string `json:"environment_id"`
	OwnershipGeneration uint64 `json:"ownership_generation"`
	Status              string `json:"status"`
}
type AuthSDKDevice struct {
	AccountID         string `json:"account_id"`
	DeviceID          string `json:"device_id"`
	AuthorityRevision uint64 `json:"authority_revision"`
	Revoked           bool   `json:"revoked"`
}
type AuthSDKKey struct {
	AccountID           string `json:"account_id"`
	DeviceID            string `json:"device_id"`
	KeyID               string `json:"key_id"`
	ApplicationID       string `json:"application_id"`
	EnvironmentID       string `json:"environment_id"`
	Generation          uint64 `json:"generation"`
	Status              string `json:"status"`
	Revoked             bool   `json:"revoked"`
	NotBeforeUnixMillis int64  `json:"not_before_unix_millis"`
	NotAfterUnixMillis  int64  `json:"not_after_unix_millis"`
}
type AuthSDKSession struct {
	AccountID           string `json:"account_id"`
	DeviceID            string `json:"device_id"`
	OwnershipGeneration uint64 `json:"ownership_generation"`
	UntilUnixMillis     int64  `json:"until_unix_millis"`
}
type AuthSDKBinding struct {
	RequestID             string   `json:"request_id"`
	SourceAccountID       string   `json:"source_account_id"`
	DeviceID              string   `json:"device_id"`
	SourceGeneration      uint64   `json:"source_generation"`
	ApplicationID         string   `json:"application_id"`
	EnvironmentID         string   `json:"environment_id"`
	BindingIntent         bool     `json:"binding_intent"`
	BindingID             string   `json:"binding_id"`
	Status                string   `json:"status"`
	AuthorityRevision     uint64   `json:"authority_revision"`
	TargetAccountID       string   `json:"target_account_id"`
	TargetProfileID       string   `json:"target_profile_id"`
	TargetEpoch           uint64   `json:"target_epoch"`
	ProfileRevision       uint64   `json:"profile_revision"`
	Scopes                []string `json:"scopes"`
	PolicyRevision        uint64   `json:"policy_revision"`
	ConsentRevision       uint64   `json:"consent_revision"`
	LinkedUntilUnixMillis int64    `json:"linked_until_unix_millis"`
}
type AuthSDKConversion struct {
	OperationID      string `json:"operation_id"`
	SourceAccountID  string `json:"source_account_id"`
	DeviceID         string `json:"device_id"`
	ApplicationID    string `json:"application_id"`
	EnvironmentID    string `json:"environment_id"`
	SourceGeneration uint64 `json:"source_generation"`
	BindingID        string `json:"binding_id"`
	Mode             string `json:"mode"`
	State            string `json:"state"`
	Revision         uint64 `json:"revision"`
	TargetAccountID  string `json:"target_account_id"`
	TargetProfileID  string `json:"target_profile_id"`
	TargetEpoch      uint64 `json:"target_epoch"`
	ProfileRevision  uint64 `json:"profile_revision"`
}
type AuthSDKMessageGrant struct {
	SourceAccountID        string   `json:"source_account_id"`
	AuthorizationRequestID string   `json:"authorization_request_id"`
	GrantID                string   `json:"grant_id"`
	ApplicationID          string   `json:"application_id"`
	EnvironmentID          string   `json:"environment_id"`
	TargetAccountID        string   `json:"target_account_id"`
	TargetProfileID        string   `json:"target_profile_id"`
	TargetEpoch            uint64   `json:"target_epoch"`
	BindingID              string   `json:"binding_id"`
	ConsentRevision        uint64   `json:"consent_revision"`
	Scopes                 []string `json:"scopes"`
	PolicyRevision         uint64   `json:"policy_revision"`
	ProfileRevision        uint64   `json:"profile_revision"`
	Status                 string   `json:"status"`
	AuthorityRevision      uint64   `json:"authority_revision"`
}

func DecodeAuthState(raw []byte) (AuthState, error) {
	var state AuthState
	bad := errors.New("invalid canonical Auth source state")
	if len(raw) > MaxStateBytes || json.Unmarshal(raw, &state) != nil {
		return AuthState{}, bad
	}
	canonical, err := json.Marshal(state)
	if err != nil || !bytes.Equal(raw, canonical) || state.SchemaVersion != SchemaVersion || !authPositive(state.Revision) ||
		state.Accounts == nil || state.SDKIdentities == nil || state.SDKDevices == nil || state.SDKKeys == nil || state.SDKSessions == nil || state.SDKBindings == nil || state.SDKConversions == nil || state.SDKMessageGrants == nil {
		return AuthState{}, bad
	}
	for _, size := range []int{len(state.Accounts), len(state.SDKIdentities), len(state.SDKDevices), len(state.SDKKeys), len(state.SDKSessions), len(state.SDKBindings), len(state.SDKConversions), len(state.SDKMessageGrants)} {
		if size > MaxSubjects {
			return AuthState{}, bad
		}
	}
	if len(state.Accounts) != len(state.SDKIdentities) {
		return AuthState{}, bad
	}
	sdk := map[string]AuthSDKIdentity{}
	actors := map[string]bool{}
	last := ""
	for i, a := range state.Accounts {
		identity := state.SDKIdentities[i]
		if !canonicalID(a.AccountID) || a.AccountID <= last || identity.AccountID != a.AccountID || a.Exists && identity.Exists {
			return AuthState{}, bad
		}
		last = a.AccountID
		if a.Exists {
			if !authOneOf(a.Type, "regular", "guest") || !authOneOf(a.Status, "active", "suspended", "deleted") || !authPositive(a.SessionEpoch, a.SecurityRevision) {
				return AuthState{}, bad
			}
		} else if a.Type != "" || a.Status != "" || a.Deleted || a.EmailPending || a.SessionEpoch != 0 || a.SecurityRevision != 0 {
			return AuthState{}, bad
		}
		if identity.Exists {
			if !authIDs(identity.ActorID, identity.ApplicationID, identity.EnvironmentID) || !authPositive(identity.OwnershipGeneration) || !authOneOf(identity.Status, "active", "suspended", "deleted", "retired") || actors[identity.ActorID] {
				return AuthState{}, bad
			}
			actors[identity.ActorID] = true
			sdk[a.AccountID] = identity
		} else if identity.ActorID != "" || identity.ApplicationID != "" || identity.EnvironmentID != "" || identity.Status != "" || identity.OwnershipGeneration != 0 {
			return AuthState{}, bad
		}
	}
	devices := map[string]AuthSDKDevice{}
	last = ""
	for _, d := range state.SDKDevices {
		key := d.AccountID + "/" + d.DeviceID
		if !authIDs(d.AccountID, d.DeviceID) || !authPositive(d.AuthorityRevision) || !sdk[d.AccountID].Exists || key <= last || devices[d.DeviceID].DeviceID != "" {
			return AuthState{}, bad
		}
		last = key
		devices[d.DeviceID] = d
	}
	parent := func(account, device, app, env string) bool {
		identity := sdk[account]
		d := devices[device]
		return authIDs(account, device, app, env) && identity.Exists && d.AccountID == account && identity.ApplicationID == app && identity.EnvironmentID == env
	}
	last = ""
	keys := map[string]bool{}
	for _, k := range state.SDKKeys {
		key := k.AccountID + "/" + k.DeviceID + "/" + k.KeyID
		if !parent(k.AccountID, k.DeviceID, k.ApplicationID, k.EnvironmentID) || !canonicalID(k.KeyID) || keys[k.KeyID] || key <= last || !authPositive(k.Generation) || k.NotBeforeUnixMillis <= 0 || k.NotAfterUnixMillis <= k.NotBeforeUnixMillis || !authOneOf(k.Status, "active", "overlap", "revoked", "expired") {
			return AuthState{}, bad
		}
		last = key
		keys[k.KeyID] = true
	}
	var previous AuthSDKSession
	for i, s := range state.SDKSessions {
		if !authIDs(s.AccountID, s.DeviceID) || devices[s.DeviceID].AccountID != s.AccountID || !authPositive(s.OwnershipGeneration) || s.UntilUnixMillis <= 0 || i > 0 && !authSessionBefore(previous, s) {
			return AuthState{}, bad
		}
		previous = s
	}
	last = ""
	requests := map[string]string{}
	for _, b := range state.SDKBindings {
		key := b.SourceAccountID + "/" + b.RequestID
		if !parent(b.SourceAccountID, b.DeviceID, b.ApplicationID, b.EnvironmentID) || !canonicalID(b.RequestID) || requests[b.RequestID] != "" || key <= last || !authPositive(b.SourceGeneration, b.PolicyRevision) || b.AuthorityRevision > math.MaxInt64 || b.ConsentRevision > math.MaxInt64 || b.LinkedUntilUnixMillis < 0 || !authTarget(b.TargetAccountID, b.TargetProfileID, b.TargetEpoch, b.ProfileRevision) || !authScopes(b.Scopes) || !authOneOf(b.Status, "unbound", "active", "revoking", "revoked") {
			return AuthState{}, bad
		}
		if b.Status != "unbound" {
			if !canonicalID(b.BindingID) || !authPositive(b.AuthorityRevision) || !b.BindingIntent {
				return AuthState{}, bad
			}
		} else if b.BindingID != "" && !canonicalID(b.BindingID) {
			return AuthState{}, bad
		}
		last = key
		requests[b.RequestID] = b.SourceAccountID
	}
	last = ""
	operations := map[string]bool{}
	for _, c := range state.SDKConversions {
		key := c.SourceAccountID + "/" + c.OperationID
		if !parent(c.SourceAccountID, c.DeviceID, c.ApplicationID, c.EnvironmentID) || !authIDs(c.OperationID, c.BindingID) || operations[c.OperationID] || key <= last || !authPositive(c.SourceGeneration, c.Revision) || !authOneOf(c.Mode, "new", "existing") || !authOneOf(c.State, "prepared", "previewed", "confirmed", "frozen", "owners_ready", "activated", "retired", "cancelled") || !authTarget(c.TargetAccountID, c.TargetProfileID, c.TargetEpoch, c.ProfileRevision) {
			return AuthState{}, bad
		}
		last = key
		operations[c.OperationID] = true
	}
	last = ""
	grants := map[string]bool{}
	for _, g := range state.SDKMessageGrants {
		key := g.SourceAccountID + "/" + g.GrantID
		identity := sdk[g.SourceAccountID]
		if !authIDs(g.SourceAccountID, g.AuthorizationRequestID, g.GrantID, g.ApplicationID, g.EnvironmentID, g.TargetAccountID, g.TargetProfileID, g.BindingID) || !identity.Exists || g.ApplicationID != identity.ApplicationID || g.EnvironmentID != identity.EnvironmentID || requests[g.AuthorizationRequestID] != g.SourceAccountID || grants[g.GrantID] || key <= last || !authPositive(g.TargetEpoch, g.ConsentRevision, g.PolicyRevision, g.ProfileRevision, g.AuthorityRevision) || !authScopes(g.Scopes) || !authOneOf(g.Status, "active", "revoking", "revoked") {
			return AuthState{}, bad
		}
		last = key
		grants[g.GrantID] = true
	}
	return state, nil
}
func authPositive(values ...uint64) bool {
	for _, v := range values {
		if v == 0 || v > math.MaxInt64 {
			return false
		}
	}
	return true
}
func authIDs(values ...string) bool {
	for _, v := range values {
		if !canonicalID(v) {
			return false
		}
	}
	return true
}
func authOneOf(value string, allowed ...string) bool {
	for _, a := range allowed {
		if a == value {
			return true
		}
	}
	return false
}
func authTarget(account, profile string, epoch, revision uint64) bool {
	if account == "" {
		return profile == "" && epoch == 0 && revision == 0
	}
	return authIDs(account, profile) && authPositive(epoch, revision)
}

var authScopePattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)

func authScopes(scopes []string) bool {
	if len(scopes) == 0 || len(scopes) > 64 {
		return false
	}
	last := ""
	for _, scope := range scopes {
		if !authScopePattern.MatchString(scope) || scope <= last {
			return false
		}
		last = scope
	}
	return true
}
func authSessionBefore(a, b AuthSDKSession) bool {
	if a.AccountID != b.AccountID {
		return a.AccountID < b.AccountID
	}
	if a.DeviceID != b.DeviceID {
		return a.DeviceID < b.DeviceID
	}
	if a.OwnershipGeneration != b.OwnershipGeneration {
		return a.OwnershipGeneration < b.OwnershipGeneration
	}
	return a.UntilUnixMillis < b.UntilUnixMillis
}
func (state AuthState) AccountActive(account string, epoch uint64) bool {
	for _, a := range state.Accounts {
		if a.AccountID == account {
			return epoch > 0 && a.Exists && a.Status == "active" && !a.Deleted && !a.EmailPending && a.SessionEpoch == epoch
		}
	}
	return false
}

// ValidUntilUnixMillis returns the next raw time boundary. Publishers also cap
// their lease before the first source read and must recheck exact caller state.
func (state AuthState) ValidUntilUnixMillis(now int64) int64 {
	earliest := int64(0)
	check := func(value int64) {
		if value > now && (earliest == 0 || value < earliest) {
			earliest = value
		}
	}
	for _, k := range state.SDKKeys {
		check(k.NotBeforeUnixMillis)
		check(k.NotAfterUnixMillis)
	}
	for _, s := range state.SDKSessions {
		check(s.UntilUnixMillis)
	}
	for _, b := range state.SDKBindings {
		check(b.LinkedUntilUnixMillis)
	}
	return earliest
}
