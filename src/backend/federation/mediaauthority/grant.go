// Package mediaauthority binds SFU admission to master-signed node authority.
package mediaauthority

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"voice/backend/federation/nodecache"
	"voice/backend/federation/protocol"
)

var ErrDenied = errors.New("federated media admission denied")

const (
	GrantClaim       = "voice_media_grant"
	grantDomain      = "voice.federation.v1.MediaAdmissionGrant\x00"
	MaxGrantValidity = 30 * time.Second
	maxTokenBytes    = 4096
)

// Grant is an admission credential, never an authority lease. Active media
// continues only while the independently verified Space snapshot/lease allows
// this account/profile/resource/session tuple. LiveKit token refresh cannot
// refresh either this credential or the master authority policy.
type Grant struct {
	Version           int    `json:"version"`
	Issuer            string `json:"issuer"`
	Audience          string `json:"audience"`
	Environment       string `json:"environment"`
	NodeID            string `json:"node_id"`
	SpaceID           string `json:"space_id"`
	Generation        int64  `json:"generation"`
	AuthorityEpoch    int64  `json:"authority_epoch"`
	AccountID         string `json:"account_id"`
	ProfileID         string `json:"profile_id"`
	ResourceID        string `json:"resource_id"`
	SessionEpoch      int64  `json:"session_epoch"`
	RoutingGeneration int64  `json:"routing_generation"`
	Nonce             string `json:"nonce"`
	ApplicationID     string `json:"application_id,omitempty"`
	EnvironmentID     string `json:"environment_id,omitempty"`
	BindingID         string `json:"binding_id,omitempty"`
	InstallationID    string `json:"installation_id,omitempty"`
	RoomName          string `json:"room_name"`
	CanPublish        bool   `json:"can_publish"`
	IssuedAt          int64  `json:"issued_at"`
	ExpiresAt         int64  `json:"expires_at"`
}

func (g Grant) Scope() protocol.Scope {
	return protocol.Scope{Issuer: g.Issuer, Environment: g.Environment, NodeID: g.NodeID, SpaceID: g.SpaceID, Generation: g.Generation, Epoch: g.AuthorityEpoch}
}

type Verifier struct {
	Issuer      string
	Environment string
	NodeID      string
	Keys        map[string]ed25519.PublicKey
}

func Sign(private ed25519.PrivateKey, keyID string, grant Grant, now time.Time) (string, error) {
	if len(private) != ed25519.PrivateKeySize || !safeText(keyID, 128) || grant.validate(now, 0) != nil {
		return "", ErrDenied
	}
	raw, err := json.Marshal(grant)
	if err != nil {
		return "", ErrDenied
	}
	envelope := protocol.Envelope{KeyID: keyID, Payload: base64.RawURLEncoding.EncodeToString(raw), Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, append([]byte(grantDomain), raw...)))}
	wire, err := json.Marshal(envelope)
	if err != nil {
		return "", ErrDenied
	}
	return base64.RawURLEncoding.EncodeToString(wire), nil
}

func (v Verifier) Verify(token, room, identity string, now time.Time, uncertainty time.Duration) (Grant, error) {
	if len(token) == 0 || len(token) > maxTokenBytes || !safeText(v.Issuer, 128) || !safeText(v.Environment, 128) || !canonicalID(v.NodeID) {
		return Grant{}, ErrDenied
	}
	wire, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return Grant{}, ErrDenied
	}
	var envelope protocol.Envelope
	if !canonicalJSON(wire, &envelope) {
		return Grant{}, ErrDenied
	}
	key, ok := v.Keys[envelope.KeyID]
	if !ok || len(key) != ed25519.PublicKeySize {
		return Grant{}, ErrDenied
	}
	raw, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if err != nil {
		return Grant{}, ErrDenied
	}
	signature, err := base64.RawURLEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(key, append([]byte(grantDomain), raw...), signature) {
		return Grant{}, ErrDenied
	}
	var grant Grant
	if !canonicalJSON(raw, &grant) || grant.validate(now, uncertainty) != nil || grant.Issuer != v.Issuer || grant.Environment != v.Environment || grant.NodeID != v.NodeID || grant.RoomName != room || grant.ProfileID != identity {
		return Grant{}, ErrDenied
	}
	return grant, nil
}

func (g Grant) validate(now time.Time, uncertainty time.Duration) error {
	if uncertainty < 0 || uncertainty > nodecache.MaxClockUncertainty || g.Version != 1 || g.Audience != "voice-node-media" || !safeText(g.Issuer, 128) || !safeText(g.Environment, 128) || !protocol.ValidRoomName(g.RoomName) ||
		!canonicalID(g.NodeID) || !canonicalID(g.SpaceID) || !canonicalID(g.AccountID) || !canonicalID(g.ProfileID) || !canonicalID(g.ResourceID) || !canonicalID(g.Nonce) || g.Generation < 1 || g.AuthorityEpoch < 1 || g.SessionEpoch < 1 || g.RoutingGeneration < 1 ||
		g.IssuedAt <= 0 || g.IssuedAt > now.UnixMilli() || g.ExpiresAt <= g.IssuedAt || g.ExpiresAt-g.IssuedAt > MaxGrantValidity.Milliseconds() || now.Add(uncertainty).UnixMilli() >= g.ExpiresAt {
		return ErrDenied
	}
	if !protocol.ValidApplicationScope(g.ApplicationID, g.EnvironmentID, g.BindingID, g.InstallationID) {
		return ErrDenied
	}
	return nil
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func safeText(value string, maxBytes int) bool {
	if value == "" || len(value) > maxBytes || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// This versioned service wire uses the deterministic JSON emitted by Sign.
// Exact round-trip equality rejects unknown/duplicate/case-aliased fields,
// trailing input and ambiguous encodings before any authority is consumed.
func canonicalJSON(raw []byte, value any) bool {
	if json.Unmarshal(raw, value) != nil {
		return false
	}
	canonical, err := json.Marshal(value)
	return err == nil && bytes.Equal(raw, canonical)
}
