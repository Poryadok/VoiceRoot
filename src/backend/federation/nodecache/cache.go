// Package nodecache verifies and atomically activates signed authority snapshots.
package nodecache

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"voice/backend/federation/protocol"
)

var (
	ErrRevisionGap       = errors.New("authority revision gap requires full resnapshot")
	ErrRevisionConflict  = errors.New("authority revision conflicts with applied state")
	ErrNoManifest        = errors.New("authority page has no staged manifest")
	ErrInvalidStage      = errors.New("authority snapshot stage is invalid")
	ErrNoAppliedSnapshot = errors.New("no complete authority snapshot is applied")
	ErrAuthorityExpired  = errors.New("authority snapshot is expired")
	ErrClockUncertain    = errors.New("clock uncertainty exceeds authority limit")
	ErrPermissionDenied  = errors.New("authority permission denied")
)

const MaxClockUncertainty = 250 * time.Millisecond

type Scope = protocol.Scope

type Cache struct {
	mu                 sync.RWMutex
	scope              Scope
	keys               map[string]ed25519.PublicKey
	current            protocol.Snapshot
	currentHash        string
	leaseUntil         int64
	leaseDeadline      time.Time
	snapshotDeadline   time.Time
	expiredLeaseUntil  int64
	snapshotExpired    bool
	announced          map[int64]string
	resnapshotRequired bool
	stage              *stagedSnapshot
}

type stagedSnapshot struct {
	manifest protocol.SnapshotManifest
	hash     string
	pages    map[int]protocol.SnapshotPage
}

func New(scope Scope, keys map[string]ed25519.PublicKey) *Cache {
	trusted := make(map[string]ed25519.PublicKey, len(keys))
	for keyID, key := range keys {
		trusted[keyID] = append(ed25519.PublicKey(nil), key...)
	}
	return &Cache{scope: scope, keys: trusted, announced: make(map[int64]string)}
}

// Current returns a detached copy of the last fully verified policy snapshot.
func (c *Cache) Current() protocol.Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneSnapshot(c.current)
}

// Authorize enforces an action against the last complete signed snapshot.
// Clock uncertainty is subtracted from the authority deadline, so it can only
// cause early denial. A partial refresh never changes the policy being read.
func (c *Cache) Authorize(accountID, profileID, resourceID string, sessionEpoch int64, action string, now time.Time, uncertainty time.Duration) error {
	return c.authorize(protocol.Permission{AccountID: accountID, ProfileID: profileID, ResourceID: resourceID, SessionEpoch: sessionEpoch}, false, action, now, uncertainty)
}

// AuthorizeScoped compares the complete route/application tuple while holding
// the policy lock. It does not clone or expose the whole policy on each check.
func (c *Cache) AuthorizeScoped(expected protocol.Permission, action string, now time.Time, uncertainty time.Duration) error {
	if expected.RoutingGeneration < 1 || !protocol.ValidApplicationScope(expected.ApplicationID, expected.EnvironmentID, expected.BindingID, expected.InstallationID) || (action == "media" && !protocol.ValidRoomName(expected.RoomName)) {
		return ErrPermissionDenied
	}
	return c.authorize(expected, true, action, now, uncertainty)
}

func (c *Cache) authorize(expected protocol.Permission, scoped bool, action string, now time.Time, uncertainty time.Duration) error {
	if c == nil || uncertainty < 0 || uncertainty > MaxClockUncertainty {
		return ErrClockUncertain
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	snapshot := c.current
	leaseUntil := c.leaseUntil
	if snapshot.Revision < 1 {
		return ErrNoAppliedSnapshot
	}
	// Time.Now carries a monotonic reading. Translating each signed wall-clock
	// deadline once prevents a subsequent wall-clock adjustment from extending
	// authority. Observed expiry also stays closed for callers without a
	// monotonic reading; clock uncertainty alone never makes that fence permanent.
	if now.UnixMilli() >= snapshot.ValidUntil || !now.Before(c.snapshotDeadline) {
		c.snapshotExpired = true
	}
	if leaseUntil != 0 && (now.UnixMilli() >= leaseUntil || !now.Before(c.leaseDeadline)) {
		c.expiredLeaseUntil = max(c.expiredLeaseUntil, leaseUntil)
	}
	if leaseUntil == 0 || c.snapshotExpired || leaseUntil <= c.expiredLeaseUntil ||
		now.Add(uncertainty).UnixMilli() >= min(snapshot.ValidUntil, leaseUntil) ||
		!now.Add(uncertainty).Before(c.snapshotDeadline) || !now.Add(uncertainty).Before(c.leaseDeadline) {
		return ErrAuthorityExpired
	}
	for _, permission := range snapshot.Permissions {
		if permission.AccountID != expected.AccountID || permission.ProfileID != expected.ProfileID || permission.ResourceID != expected.ResourceID || permission.SessionEpoch != expected.SessionEpoch {
			continue
		}
		if scoped && (permission.RoutingGeneration != expected.RoutingGeneration || permission.RoomName != expected.RoomName || permission.ApplicationID != expected.ApplicationID || permission.EnvironmentID != expected.EnvironmentID || permission.BindingID != expected.BindingID || permission.InstallationID != expected.InstallationID) {
			continue
		}
		for _, granted := range permission.Actions {
			if granted == action {
				return nil
			}
		}
	}
	return ErrPermissionDenied
}

// AcceptLease records only a signed lease for the exact fully applied policy.
// Applying a newer snapshot clears the old lease until the new revision is ACKed.
func (c *Cache) AcceptLease(envelope protocol.Envelope, now time.Time) error {
	claims, err := c.verify(envelope, now)
	if err != nil {
		return err
	}
	if claims.Kind != "lease" {
		return ErrInvalidStage
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if claims.Revision != c.current.Revision || claims.Hash != c.currentHash || claims.ExpiresAt > c.current.ValidUntil {
		return ErrRevisionConflict
	}
	if c.snapshotExpired || claims.ExpiresAt <= c.expiredLeaseUntil {
		return ErrAuthorityExpired
	}
	if claims.ExpiresAt == c.leaseUntil {
		// An exact replay cannot translate the same deadline a second time.
		if !now.Before(c.leaseDeadline) {
			c.expiredLeaseUntil = max(c.expiredLeaseUntil, c.leaseUntil)
			return ErrAuthorityExpired
		}
		return nil
	}
	if claims.ExpiresAt < c.leaseUntil {
		return ErrRevisionConflict
	}
	c.leaseUntil = claims.ExpiresAt
	c.leaseDeadline = now.Add(time.Duration(claims.ExpiresAt-now.UnixMilli()) * time.Millisecond)
	return nil
}

// StageManifest starts or resumes an isolated page stage. It never changes the
// policy visible through Current until every page and the whole-object digest
// have been verified.
func (c *Cache) StageManifest(envelope protocol.Envelope, now time.Time) error {
	claims, err := c.verify(envelope, now)
	if err != nil {
		return err
	}
	if claims.Kind != "snapshot_manifest" || claims.Manifest == nil {
		return ErrInvalidStage
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	manifest := *claims.Manifest
	if claims.Revision == c.current.Revision {
		if claims.Hash == c.currentHash {
			return nil
		}
		return ErrRevisionConflict
	}
	if claims.Revision < c.current.Revision {
		return ErrRevisionConflict
	}
	if expectedHash, announced := c.announced[claims.Revision]; announced && expectedHash != claims.Hash {
		return ErrRevisionConflict
	}
	if c.current.Revision > 0 && claims.Revision != c.current.Revision+1 && !c.resnapshotRequired {
		return ErrRevisionGap
	}
	if c.stage != nil {
		if c.stage.manifest.Revision != manifest.Revision || c.stage.hash != claims.Hash || !sameManifest(c.stage.manifest, manifest) {
			return ErrRevisionConflict
		}
		return nil
	}
	c.stage = &stagedSnapshot{manifest: manifest, hash: claims.Hash, pages: make(map[int]protocol.SnapshotPage, manifest.PageCount)}
	return nil
}

// StagePage verifies one signed page. Pages may arrive in any order; duplicate
// exact pages are inert and a conflicting duplicate discards the partial stage.
func (c *Cache) StagePage(envelope protocol.Envelope, now time.Time) error {
	claims, err := c.verify(envelope, now)
	if err != nil {
		return err
	}
	if claims.Kind != "snapshot_page" || claims.Page == nil {
		return ErrInvalidStage
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	stage := c.stage
	if stage == nil {
		return ErrNoManifest
	}
	page := *claims.Page
	if claims.Revision != stage.manifest.Revision || claims.Hash != stage.hash || page.PageCount != stage.manifest.PageCount || claims.ExpiresAt != stage.manifest.ValidUntil || page.TotalHash != stage.hash {
		c.stage = nil
		return ErrInvalidStage
	}
	if previous, exists := stage.pages[page.PageIndex]; exists {
		if !samePage(previous, page) {
			c.stage = nil
			return ErrRevisionConflict
		}
		return nil
	}
	page.Permissions = clonePermissions(page.Permissions)
	stage.pages[page.PageIndex] = page
	if len(stage.pages) != stage.manifest.PageCount {
		return nil
	}
	permissions := make([]protocol.Permission, 0, stage.manifest.PageCount*protocol.SnapshotPagePermissionLimit)
	for index := 0; index < stage.manifest.PageCount; index++ {
		stored, ok := stage.pages[index]
		if !ok {
			return nil
		}
		permissions = append(permissions, stored.Permissions...)
	}
	snapshot := protocol.Snapshot{Version: 1, Complete: true, PageCount: stage.manifest.PageCount, Revision: stage.manifest.Revision, ValidUntil: stage.manifest.ValidUntil, Permissions: permissions}
	if snapshot.Validate(now) != nil || protocol.SnapshotDigest(snapshot) != stage.hash {
		c.stage = nil
		return ErrInvalidStage
	}
	c.current = cloneSnapshot(snapshot)
	c.currentHash = stage.hash
	c.leaseUntil = 0
	c.leaseDeadline = time.Time{}
	c.snapshotDeadline = now.Add(time.Duration(snapshot.ValidUntil-now.UnixMilli()) * time.Millisecond)
	c.expiredLeaseUntil = 0
	c.snapshotExpired = false
	for revision := range c.announced {
		if revision <= snapshot.Revision {
			delete(c.announced, revision)
		}
	}
	c.resnapshotRequired = false
	c.stage = nil
	return nil
}

// ObserveRevisionStream verifies an ordered stream cursor. Events signal when
// policy changed but never mutate active permissions; every changed revision
// still needs a complete signed page set.
func (c *Cache) ObserveRevisionStream(envelope protocol.Envelope, now time.Time) error {
	claims, err := c.verify(envelope, now)
	if err != nil {
		return err
	}
	if claims.Kind != "revision_stream" || claims.RevisionStream == nil {
		return ErrInvalidStage
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	currentRevision := c.current.Revision
	stream := claims.RevisionStream
	if stream.AfterRevision != currentRevision || len(stream.Events) > 100 {
		c.stage = nil
		c.resnapshotRequired = true
		clear(c.announced)
		return ErrRevisionGap
	}
	if stream.ResnapshotRequired {
		c.stage = nil
		c.resnapshotRequired = true
		clear(c.announced)
		return ErrRevisionGap
	}
	expected := currentRevision + 1
	for _, event := range stream.Events {
		if event.Revision != expected {
			c.stage = nil
			c.resnapshotRequired = true
			clear(c.announced)
			return ErrRevisionGap
		}
		expected++
	}
	if expected-1 != stream.CurrentRevision {
		c.stage = nil
		c.resnapshotRequired = true
		clear(c.announced)
		return ErrRevisionGap
	}
	for _, event := range stream.Events {
		if expectedHash, exists := c.announced[event.Revision]; exists && expectedHash != event.Hash {
			return ErrRevisionConflict
		}
		c.announced[event.Revision] = event.Hash
	}
	return nil
}

// LeaseAck is emitted only for a fully activated revision and is intended for
// the node's short-lived bearer lease request.
func (c *Cache) LeaseAck(nonce string) (protocol.AppliedRevisionAck, error) {
	if id, err := uuid.Parse(nonce); err != nil || id == uuid.Nil || id.String() != nonce {
		return protocol.AppliedRevisionAck{}, ErrInvalidStage
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.current.Revision < 1 || c.currentHash == "" {
		return protocol.AppliedRevisionAck{}, ErrNoAppliedSnapshot
	}
	return protocol.AppliedRevisionAck{Revision: c.current.Revision, Hash: c.currentHash, Nonce: nonce}, nil
}

func (c *Cache) verify(envelope protocol.Envelope, now time.Time) (protocol.Claims, error) {
	key, ok := c.keys[envelope.KeyID]
	if !ok {
		return protocol.Claims{}, protocol.ErrForbidden
	}
	claims, err := protocol.VerifyEnvelope(key, envelope, now)
	if err != nil {
		return protocol.Claims{}, err
	}
	if err := protocol.VerifyScope(claims, c.scope); err != nil {
		return protocol.Claims{}, err
	}
	return claims, nil
}

func sameManifest(a, b protocol.SnapshotManifest) bool {
	return a.Version == b.Version && a.Complete == b.Complete && a.PageCount == b.PageCount && a.Revision == b.Revision && a.ValidUntil == b.ValidUntil && a.TotalHash == b.TotalHash
}

func samePage(a, b protocol.SnapshotPage) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func cloneSnapshot(snapshot protocol.Snapshot) protocol.Snapshot {
	if snapshot.Permissions == nil {
		return snapshot
	}
	snapshot.Permissions = clonePermissions(snapshot.Permissions)
	return snapshot
}

func clonePermissions(permissions []protocol.Permission) []protocol.Permission {
	cloned := make([]protocol.Permission, len(permissions))
	for i, permission := range permissions {
		cloned[i] = permission
		cloned[i].Actions = append([]string(nil), permission.Actions...)
	}
	return cloned
}
