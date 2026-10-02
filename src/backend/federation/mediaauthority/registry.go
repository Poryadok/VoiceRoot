package mediaauthority

import (
	"crypto/ed25519"
	"sync"
	"sync/atomic"
	"time"

	"voice/backend/federation/nodecache"
	"voice/backend/federation/protocol"
)

// Bundle carries untrusted transport data. Every envelope must be signed for
// the configured node and exact Space scope before the complete policy activates.
type Bundle struct {
	Scope    protocol.Scope      `json:"scope"`
	Manifest protocol.Envelope   `json:"manifest"`
	Pages    []protocol.Envelope `json:"pages"`
	Lease    protocol.Envelope   `json:"lease"`
}

type spacePolicy struct {
	scope protocol.Scope
	cache *nodecache.Cache
}

type Registry struct {
	mu            sync.RWMutex
	verifier      Verifier
	skew          time.Duration
	spaces        map[string]spacePolicy
	wallHighWater atomic.Int64
}

// Admission cannot be constructed by a caller. The SFU saves the verified
// admission against its concrete participant ID, then rechecks active policy
// independently of the admission credential's expiry or JWT refresh.
type Admission struct {
	registry *Registry
	grant    Grant
}

// CanPublish is a verified master restriction, independent of the node's JWT.
func (a Admission) CanPublish() bool { return a.registry != nil && a.grant.CanPublish }

func NewRegistry(verifier Verifier, uncertainty time.Duration) (*Registry, error) {
	if !safeText(verifier.Issuer, 128) || !safeText(verifier.Environment, 128) || !canonicalID(verifier.NodeID) ||
		uncertainty < 0 || uncertainty > nodecache.MaxClockUncertainty || len(verifier.Keys) == 0 {
		return nil, ErrDenied
	}
	keys := make(map[string]ed25519.PublicKey, len(verifier.Keys))
	for id, key := range verifier.Keys {
		if !safeText(id, 128) || len(key) != ed25519.PublicKeySize {
			return nil, ErrDenied
		}
		keys[id] = append(ed25519.PublicKey(nil), key...)
	}
	verifier.Keys = keys
	return &Registry{verifier: verifier, skew: uncertainty, spaces: make(map[string]spacePolicy)}, nil
}

func (r *Registry) Apply(bundle Bundle, now time.Time) error {
	if r == nil {
		return ErrDenied
	}
	checkedNow, ok := r.observeTime(now)
	if !ok || bundle.Scope.Issuer != r.verifier.Issuer || bundle.Scope.Environment != r.verifier.Environment ||
		bundle.Scope.NodeID != r.verifier.NodeID || !canonicalID(bundle.Scope.SpaceID) || bundle.Scope.Generation < 1 || bundle.Scope.Epoch < 1 ||
		len(bundle.Pages) < 1 || len(bundle.Pages) > protocol.SnapshotMaxPageCount {
		return ErrDenied
	}
	now = checkedNow
	candidate := nodecache.New(bundle.Scope, r.verifier.Keys)
	if candidate.StageManifest(bundle.Manifest, now) != nil {
		return ErrDenied
	}
	for _, page := range bundle.Pages {
		if candidate.StagePage(page, now) != nil {
			return ErrDenied
		}
	}
	if candidate.AcceptLease(bundle.Lease, now) != nil {
		return ErrDenied
	}
	snapshot := candidate.Current()
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, exists := r.spaces[bundle.Scope.SpaceID]
	if exists {
		if bundle.Scope.Generation < previous.scope.Generation || bundle.Scope.Epoch < previous.scope.Epoch {
			return ErrDenied
		}
		if bundle.Scope == previous.scope {
			applied := previous.cache.Current()
			if snapshot.Revision < applied.Revision {
				return ErrDenied
			}
			if snapshot.Revision == applied.Revision {
				if protocol.SnapshotDigest(snapshot) != protocol.SnapshotDigest(applied) || previous.cache.AcceptLease(bundle.Lease, now) != nil {
					return ErrDenied
				}
				return nil
			}
		}
	}
	// A complete higher revision is a full resnapshot, never an event-only patch.
	r.spaces[bundle.Scope.SpaceID] = spacePolicy{scope: bundle.Scope, cache: candidate}
	return nil
}

func (r *Registry) Admit(token, room, identity string, now time.Time) (Admission, error) {
	if r == nil {
		return Admission{}, ErrDenied
	}
	checkedNow, ok := r.observeTime(now)
	if !ok {
		return Admission{}, ErrDenied
	}
	now = checkedNow
	grant, err := r.verifier.Verify(token, room, identity, now, r.skew)
	if err != nil {
		return Admission{}, ErrDenied
	}
	admission := Admission{registry: r, grant: grant}
	if r.Check(admission, now) != nil {
		return Admission{}, ErrDenied
	}
	return admission, nil
}

func (r *Registry) Check(admission Admission, now time.Time) error {
	if r == nil || admission.registry != r {
		return ErrDenied
	}
	checkedNow, ok := r.observeTime(now)
	if !ok {
		return ErrDenied
	}
	now = checkedNow
	r.mu.RLock()
	defer r.mu.RUnlock()
	grant := admission.grant
	policy, ok := r.spaces[grant.SpaceID]
	expected := protocol.Permission{AccountID: grant.AccountID, ProfileID: grant.ProfileID, ResourceID: grant.ResourceID, SessionEpoch: grant.SessionEpoch, RoutingGeneration: grant.RoutingGeneration, RoomName: grant.RoomName,
		ApplicationID: grant.ApplicationID, EnvironmentID: grant.EnvironmentID, BindingID: grant.BindingID, InstallationID: grant.InstallationID}
	if !ok || policy.scope != grant.Scope() ||
		policy.cache.AuthorizeScoped(expected, "media", now, r.skew) != nil ||
		(grant.CanPublish && policy.cache.AuthorizeScoped(expected, "media_publish", now, r.skew) != nil) {
		return ErrDenied
	}
	return nil
}

// Once a receiver has observed later wall time, older wall time cannot make
// a saved higher revision appear fresh. Concurrent samples within the clock
// bound use the conservative later instant, which cannot extend authority.
// Larger rollback closes media until the clock catches up.
func (r *Registry) observeTime(now time.Time) (time.Time, bool) {
	wall := now.UnixMilli()
	for {
		previous := r.wallHighWater.Load()
		if wall < previous {
			delta := time.Duration(previous-wall) * time.Millisecond
			if delta > r.skew {
				return time.Time{}, false
			}
			return now.Add(delta), true
		}
		if wall == previous || r.wallHighWater.CompareAndSwap(previous, wall) {
			return now, true
		}
	}
}
