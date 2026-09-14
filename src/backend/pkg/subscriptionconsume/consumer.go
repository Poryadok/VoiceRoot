// Package subscriptionconsume holds the local, fail-closed personal entitlement
// projection used by File. It deliberately has no legacy event consumer: only a
// complete revisioned entitlement snapshot may update this projection.
package subscriptionconsume

import (
	"errors"
	"strings"
	"sync"
	"time"
)

var (
	// ErrInvalidSnapshot means the candidate cannot safely update a projection.
	ErrInvalidSnapshot = errors.New("subscription entitlement: invalid snapshot")
	// ErrConflictingRevision means a revision was reused with different facts.
	ErrConflictingRevision = errors.New("subscription entitlement: conflicting revision")
)

// EntitlementState is the canonical personal entitlement state.
type EntitlementState string

const (
	StateActive      EntitlementState = "ACTIVE"
	StateGracePeriod EntitlementState = "GRACE_PERIOD"
	StateInactive    EntitlementState = "INACTIVE"
)

// EntitlementSnapshot is the pre-wire local state core of a personal entitlement
// fact. The eventual enforcement consumer additionally validates the complete
// envelope, event ID/hash, inbox and snapshot-reconciliation protocol.
type EntitlementSnapshot struct {
	AccountID     string
	Revision      uint64
	State         EntitlementState
	EntitledUntil time.Time
}

// ApplyResult describes whether a snapshot changed the projection.
type ApplyResult uint8

const (
	ApplyApplied ApplyResult = iota + 1
	ApplyDuplicate
	ApplyStale
	ApplyConflict
	ApplyInvalid
)

// Err exposes the actionable error category for rejected snapshots.
func (r ApplyResult) Err() error {
	switch r {
	case ApplyConflict:
		return ErrConflictingRevision
	case ApplyInvalid:
		return ErrInvalidSnapshot
	default:
		return nil
	}
}

// TierCache stores the newest complete entitlement snapshot for each account.
type TierCache struct {
	mu    sync.RWMutex
	tiers map[string]EntitlementSnapshot
}

// NewTierCache returns an empty projection. Unknown accounts fail closed to free.
func NewTierCache() *TierCache {
	return &TierCache{tiers: make(map[string]EntitlementSnapshot)}
}

// Apply atomically applies a complete snapshot by aggregate revision. A later
// revision wins; an earlier one is stale; a conflicting same revision is rejected.
func (c *TierCache) Apply(snapshot EntitlementSnapshot) ApplyResult {
	if c == nil || !validSnapshot(snapshot) {
		return ApplyInvalid
	}
	key := normalizedAccountID(snapshot.AccountID)
	snapshot.AccountID = key
	snapshot.EntitledUntil = snapshot.EntitledUntil.UTC().Round(0)
	c.mu.Lock()
	defer c.mu.Unlock()
	current, exists := c.tiers[key]
	if !exists || snapshot.Revision > current.Revision {
		c.tiers[key] = snapshot
		return ApplyApplied
	}
	if snapshot.Revision < current.Revision {
		return ApplyStale
	}
	if snapshot == current {
		return ApplyDuplicate
	}
	return ApplyConflict
}

// TierAt returns premium only while a current ACTIVE or GRACE_PERIOD snapshot
// grants it. Equality at entitled_until is deliberately free.
func (c *TierCache) TierAt(accountID string, now time.Time) string {
	if c == nil {
		return "free"
	}
	key := normalizedAccountID(accountID)
	if key == "" {
		return "free"
	}
	c.mu.RLock()
	snapshot, ok := c.tiers[key]
	c.mu.RUnlock()
	if !ok || !now.Before(snapshot.EntitledUntil) {
		return "free"
	}
	switch snapshot.State {
	case StateActive, StateGracePeriod:
		return "premium"
	default:
		return "free"
	}
}

func validSnapshot(snapshot EntitlementSnapshot) bool {
	if normalizedAccountID(snapshot.AccountID) == "" || snapshot.Revision == 0 || snapshot.EntitledUntil.IsZero() {
		return false
	}
	switch snapshot.State {
	case StateActive, StateGracePeriod, StateInactive:
		return true
	default:
		return false
	}
}

func normalizedAccountID(accountID string) string {
	return strings.ToLower(strings.TrimSpace(accountID))
}
