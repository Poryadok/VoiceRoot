// Package protocol defines the signed master-to-node authority wire contract.
package protocol

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	SnapshotPagePermissionLimit = 256
	SnapshotMaxPageCount        = 4096
	MaxReceiverBootNonces       = 16
)

var ErrInvalid = errors.New("invalid authority protocol value")
var ErrForbidden = errors.New("authority protocol verification failed")

type Permission struct {
	AccountID         string   `json:"account_id"`
	ProfileID         string   `json:"profile_id"`
	ResourceID        string   `json:"resource_id"`
	SessionEpoch      int64    `json:"session_epoch"`
	Actions           []string `json:"actions"`
	RoutingGeneration int64    `json:"routing_generation,omitempty"`
	RoomName          string   `json:"room_name,omitempty"`
	ApplicationID     string   `json:"application_id,omitempty"`
	EnvironmentID     string   `json:"environment_id,omitempty"`
	BindingID         string   `json:"binding_id,omitempty"`
	InstallationID    string   `json:"installation_id,omitempty"`
}

// Application scope is absent for ordinary Voice or complete for a game-bound
// actor. Installation is optional only within that complete application scope.
func ValidApplicationScope(application, environment, binding, installation string) bool {
	if application == "" && environment == "" && binding == "" && installation == "" {
		return true
	}
	return canonicalID(application) && canonicalID(environment) && canonicalID(binding) && (installation == "" || canonicalID(installation))
}

func ValidRoomName(value string) bool {
	if value == "" || len(value) > 256 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

type Snapshot struct {
	Version     int          `json:"version"`
	Complete    bool         `json:"complete"`
	PageCount   int          `json:"page_count"`
	Revision    int64        `json:"revision"`
	ValidUntil  int64        `json:"valid_until"`
	Permissions []Permission `json:"permissions"`
}

type SnapshotManifest struct {
	Version    int    `json:"version"`
	Complete   bool   `json:"complete"`
	PageCount  int    `json:"page_count"`
	Revision   int64  `json:"revision"`
	ValidUntil int64  `json:"valid_until"`
	TotalHash  string `json:"total_hash"`
}

type SnapshotPage struct {
	Version     int          `json:"version"`
	Revision    int64        `json:"revision"`
	PageIndex   int          `json:"page_index"`
	PageCount   int          `json:"page_count"`
	PageHash    string       `json:"page_hash"`
	TotalHash   string       `json:"total_hash"`
	Permissions []Permission `json:"permissions"`
}

type RevisionEvent struct {
	Revision   int64  `json:"revision"`
	Hash       string `json:"hash"`
	ValidUntil int64  `json:"valid_until"`
}

type RevisionStream struct {
	AfterRevision      int64           `json:"after_revision"`
	CurrentRevision    int64           `json:"current_revision"`
	ResnapshotRequired bool            `json:"resnapshot_required"`
	Events             []RevisionEvent `json:"events"`
}

type Claims struct {
	Version            int               `json:"version"`
	Kind               string            `json:"kind"`
	Issuer             string            `json:"issuer"`
	Audience           string            `json:"audience"`
	Environment        string            `json:"environment"`
	NodeID             string            `json:"node_id"`
	SpaceID            string            `json:"space_id"`
	Generation         int64             `json:"generation"`
	Epoch              int64             `json:"epoch"`
	Revision           int64             `json:"revision"`
	IssuedAt           int64             `json:"issued_at"`
	ExpiresAt          int64             `json:"expires_at"`
	Hash               string            `json:"hash"`
	Snapshot           *Snapshot         `json:"snapshot,omitempty"`
	Manifest           *SnapshotManifest `json:"manifest,omitempty"`
	Page               *SnapshotPage     `json:"page,omitempty"`
	RevisionEvent      *RevisionEvent    `json:"revision_event,omitempty"`
	RevisionStream     *RevisionStream   `json:"revision_stream,omitempty"`
	ReceiverBootNonces []string          `json:"receiver_boot_nonces,omitempty"`
}

type Envelope struct {
	KeyID     string `json:"key_id"`
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type Scope struct {
	Issuer      string
	Environment string
	NodeID      string
	SpaceID     string
	Generation  int64
	Epoch       int64
}

type AppliedRevisionAck struct {
	Revision           int64    `json:"revision"`
	Hash               string   `json:"hash"`
	Nonce              string   `json:"nonce"`
	ReceiverBootNonces []string `json:"receiver_boot_nonces,omitempty"`
}

// Boot UUIDs are an exact bounded set in signed canonical order. Empty preserves
// legacy transport; production receivers require their own newly generated UUID.
func ValidReceiverBootNonces(nonces []string) bool {
	if len(nonces) > MaxReceiverBootNonces {
		return false
	}
	for i, nonce := range nonces {
		if !canonicalID(nonce) || (i > 0 && nonces[i-1] >= nonce) {
			return false
		}
	}
	return true
}

func (s Snapshot) Validate(now time.Time) error {
	pageCount := max(1, (len(s.Permissions)+SnapshotPagePermissionLimit-1)/SnapshotPagePermissionLimit)
	if s.Version != 1 || !s.Complete || s.PageCount != pageCount || s.PageCount > SnapshotMaxPageCount || s.Revision < 1 ||
		s.ValidUntil <= now.UnixMilli() || s.ValidUntil > now.Add(2*time.Second).UnixMilli() || s.Permissions == nil {
		return ErrInvalid
	}
	for _, p := range s.Permissions {
		if !canonicalID(p.AccountID) || !canonicalID(p.ProfileID) || !canonicalID(p.ResourceID) || p.SessionEpoch < 1 || len(p.Actions) == 0 {
			return ErrInvalid
		}
		media := slices.Contains(p.Actions, "media")
		if slices.Contains(p.Actions, "media_publish") && !media {
			return ErrInvalid
		}
		if !ValidApplicationScope(p.ApplicationID, p.EnvironmentID, p.BindingID, p.InstallationID) || p.RoutingGeneration < 0 || (p.RoomName != "" && (!media || !ValidRoomName(p.RoomName) || p.RoutingGeneration < 1)) || (media && p.RoutingGeneration > 0 && p.RoomName == "") {
			return ErrInvalid
		}
		for _, action := range p.Actions {
			if action != "read" && action != "write" && action != "subscribe" && action != "media" && action != "media_publish" {
				return ErrInvalid
			}
		}
	}
	return nil
}

func ManifestFor(snapshot Snapshot) SnapshotManifest {
	return SnapshotManifest{Version: 1, Complete: true, PageCount: snapshot.PageCount, Revision: snapshot.Revision, ValidUntil: snapshot.ValidUntil, TotalHash: SnapshotDigest(snapshot)}
}

func SnapshotDigest(snapshot Snapshot) string {
	raw, _ := json.Marshal(snapshot)
	return Digest(raw)
}

func EmptySnapshotDigest(revision int64, validUntil time.Time) string {
	snapshot := Snapshot{Version: 1, Complete: true, PageCount: 1, Revision: revision, ValidUntil: validUntil.UnixMilli(), Permissions: []Permission{}}
	return SnapshotDigest(snapshot)
}

func Digest(raw []byte) string {
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:])
}

func SignEnvelope(private ed25519.PrivateKey, keyID string, claims Claims) (Envelope, error) {
	if len(private) != ed25519.PrivateKeySize || keyID == "" {
		return Envelope{}, ErrInvalid
	}
	raw, err := json.Marshal(claims)
	if err != nil {
		return Envelope{}, err
	}
	return Envelope{KeyID: keyID, Payload: base64.RawURLEncoding.EncodeToString(raw), Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, raw))}, nil
}

func VerifyEnvelope(public ed25519.PublicKey, envelope Envelope, now time.Time) (Claims, error) {
	var claims Claims
	raw, err := base64.RawURLEncoding.DecodeString(envelope.Payload)
	if err != nil || len(public) != ed25519.PublicKeySize {
		return claims, ErrForbidden
	}
	signature, err := base64.RawURLEncoding.DecodeString(envelope.Signature)
	if err != nil || !ed25519.Verify(public, raw, signature) || strictJSON(raw, &claims) != nil || validateClaims(claims, now) != nil {
		return Claims{}, ErrForbidden
	}
	return claims, nil
}

func VerifyScope(claims Claims, scope Scope) error {
	if claims.Issuer != scope.Issuer || claims.Audience != "voice-node" || claims.Environment != scope.Environment ||
		claims.NodeID != scope.NodeID || claims.SpaceID != scope.SpaceID || claims.Generation != scope.Generation || claims.Epoch != scope.Epoch {
		return ErrForbidden
	}
	return nil
}

func SignSnapshotManifest(private ed25519.PrivateKey, keyID string, scope Scope, manifest SnapshotManifest, now time.Time) (Envelope, error) {
	if manifest.Version != 1 || !manifest.Complete || manifest.Revision < 1 || manifest.PageCount < 1 || manifest.PageCount > SnapshotMaxPageCount || !validDigest(manifest.TotalHash) || manifest.ValidUntil <= now.UnixMilli() || manifest.ValidUntil > now.Add(2*time.Second).UnixMilli() {
		return Envelope{}, ErrInvalid
	}
	return SignEnvelope(private, keyID, Claims{Version: 1, Kind: "snapshot_manifest", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment, NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch, Revision: manifest.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: manifest.ValidUntil, Hash: manifest.TotalHash, Manifest: &manifest})
}

func SignSnapshotPages(private ed25519.PrivateKey, keyID string, scope Scope, snapshot Snapshot, now time.Time) ([]Envelope, error) {
	if err := snapshot.Validate(now); err != nil {
		return nil, err
	}
	manifest := ManifestFor(snapshot)
	pages := make([]Envelope, 0, snapshot.PageCount)
	for index := 0; index < snapshot.PageCount; index++ {
		start := index * SnapshotPagePermissionLimit
		end := min(start+SnapshotPagePermissionLimit, len(snapshot.Permissions))
		permissions := make([]Permission, end-start)
		copy(permissions, snapshot.Permissions[start:end])
		page := SnapshotPage{Version: 1, Revision: snapshot.Revision, PageIndex: index, PageCount: snapshot.PageCount, PageHash: permissionDigest(permissions), TotalHash: manifest.TotalHash, Permissions: permissions}
		envelope, err := SignEnvelope(private, keyID, Claims{Version: 1, Kind: "snapshot_page", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment, NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch, Revision: snapshot.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: snapshot.ValidUntil, Hash: manifest.TotalHash, Page: &page})
		if err != nil {
			return nil, err
		}
		pages = append(pages, envelope)
	}
	return pages, nil
}

func SignRevisionEvent(private ed25519.PrivateKey, keyID string, scope Scope, event RevisionEvent, now time.Time) (Envelope, error) {
	if event.Revision < 1 || !validDigest(event.Hash) || event.ValidUntil <= now.UnixMilli() || event.ValidUntil > now.Add(2*time.Second).UnixMilli() {
		return Envelope{}, ErrInvalid
	}
	return SignEnvelope(private, keyID, Claims{Version: 1, Kind: "revision", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment, NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch, Revision: event.Revision, IssuedAt: now.UnixMilli(), ExpiresAt: event.ValidUntil, Hash: event.Hash, RevisionEvent: &event})
}

func SignRevisionStream(private ed25519.PrivateKey, keyID string, scope Scope, stream RevisionStream, expiresAt int64, now time.Time) (Envelope, error) {
	if stream.AfterRevision < 0 || stream.CurrentRevision < 1 || (!stream.ResnapshotRequired && stream.AfterRevision > stream.CurrentRevision) || expiresAt <= now.UnixMilli() || expiresAt > now.Add(2*time.Second).UnixMilli() {
		return Envelope{}, ErrInvalid
	}
	return SignEnvelope(private, keyID, Claims{Version: 1, Kind: "revision_stream", Issuer: scope.Issuer, Audience: "voice-node", Environment: scope.Environment, NodeID: scope.NodeID, SpaceID: scope.SpaceID, Generation: scope.Generation, Epoch: scope.Epoch, Revision: stream.CurrentRevision, IssuedAt: now.UnixMilli(), ExpiresAt: expiresAt, Hash: streamDigest(stream), RevisionStream: &stream})
}

func validateClaims(c Claims, now time.Time) error {
	if !ValidReceiverBootNonces(c.ReceiverBootNonces) || (c.Kind != "lease" && len(c.ReceiverBootNonces) > 0) {
		return ErrInvalid
	}
	if c.Version != 1 || c.Issuer == "" || c.Audience != "voice-node" || c.Environment == "" || !canonicalID(c.NodeID) || !canonicalID(c.SpaceID) || c.Generation < 1 || c.Epoch < 1 || c.Revision < 1 || c.IssuedAt > now.UnixMilli() || c.ExpiresAt <= now.UnixMilli() || c.ExpiresAt <= c.IssuedAt || !validDigest(c.Hash) {
		return ErrInvalid
	}
	switch c.Kind {
	case "lease":
		if c.ExpiresAt-c.IssuedAt > 2000 || c.Snapshot != nil || c.Manifest != nil || c.Page != nil || c.RevisionEvent != nil || c.RevisionStream != nil {
			return ErrInvalid
		}
	case "snapshot":
		if c.Snapshot == nil || c.Manifest != nil || c.Page != nil || c.RevisionEvent != nil || c.RevisionStream != nil || c.Snapshot.Revision != c.Revision || c.Snapshot.ValidUntil != c.ExpiresAt || c.Snapshot.Validate(now) != nil || SnapshotDigest(*c.Snapshot) != c.Hash {
			return ErrInvalid
		}
	case "snapshot_manifest":
		if c.Manifest == nil || c.Snapshot != nil || c.Page != nil || c.RevisionEvent != nil || c.RevisionStream != nil || c.Manifest.Revision != c.Revision || c.Manifest.ValidUntil != c.ExpiresAt || c.Manifest.TotalHash != c.Hash || c.Manifest.Version != 1 || !c.Manifest.Complete || c.Manifest.PageCount < 1 || c.Manifest.PageCount > SnapshotMaxPageCount {
			return ErrInvalid
		}
	case "snapshot_page":
		if c.Page == nil || c.Snapshot != nil || c.Manifest != nil || c.RevisionEvent != nil || c.RevisionStream != nil || c.Page.Version != 1 || c.Page.Revision != c.Revision || c.Page.PageCount < 1 || c.Page.PageCount > SnapshotMaxPageCount || c.Page.PageIndex < 0 || c.Page.PageIndex >= c.Page.PageCount || c.Page.TotalHash != c.Hash || len(c.Page.Permissions) > SnapshotPagePermissionLimit || permissionDigest(c.Page.Permissions) != c.Page.PageHash {
			return ErrInvalid
		}
	case "revision":
		if c.RevisionEvent == nil || c.Snapshot != nil || c.Manifest != nil || c.Page != nil || c.RevisionStream != nil || c.RevisionEvent.Revision != c.Revision || c.RevisionEvent.ValidUntil != c.ExpiresAt || c.RevisionEvent.Hash != c.Hash {
			return ErrInvalid
		}
	case "revision_stream":
		if c.RevisionStream == nil || c.Snapshot != nil || c.Manifest != nil || c.Page != nil || c.RevisionEvent != nil || c.RevisionStream.CurrentRevision != c.Revision || c.RevisionStream.AfterRevision < 0 || (!c.RevisionStream.ResnapshotRequired && c.RevisionStream.AfterRevision > c.RevisionStream.CurrentRevision) || streamDigest(*c.RevisionStream) != c.Hash || len(c.RevisionStream.Events) > 100 {
			return ErrInvalid
		}
		stream := c.RevisionStream
		if stream.ResnapshotRequired {
			if len(stream.Events) != 0 {
				return ErrInvalid
			}
		} else {
			expected := stream.AfterRevision + 1
			for _, event := range stream.Events {
				if event.Revision != expected || !validDigest(event.Hash) || event.ValidUntil <= 0 {
					return ErrInvalid
				}
				expected++
			}
			if expected-1 != stream.CurrentRevision {
				return ErrInvalid
			}
		}
	default:
		return ErrInvalid
	}
	return nil
}

func streamDigest(stream RevisionStream) string {
	raw, _ := json.Marshal(stream)
	return Digest(raw)
}

func permissionDigest(permissions []Permission) string {
	raw, _ := json.Marshal(permissions)
	return Digest(raw)
}

func validDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == value
}

func canonicalID(value string) bool {
	id, err := uuid.Parse(value)
	return err == nil && id != uuid.Nil && id.String() == value
}

func strictJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrInvalid
	}
	if decoder.Decode(new(any)) != io.EOF {
		return ErrInvalid
	}
	return nil
}
