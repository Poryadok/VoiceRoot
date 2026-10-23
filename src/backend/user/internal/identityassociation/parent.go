package identityassociation

import (
	"context"
	"github.com/google/uuid"
	"math"
	"strings"
	"time"
)

type ParentState string

const (
	ParentActive      ParentState = "ACTIVE"
	ParentRecoverable ParentState = "RECOVERABLE"
	ParentTerminal    ParentState = "TERMINAL"
)

type ParentEligibility string

const (
	ParentEligible   ParentEligibility = "ELIGIBLE"
	ParentIneligible ParentEligibility = "INELIGIBLE"
)

type ParentDisposition string

const (
	ParentNotIrreversible ParentDisposition = "NOT_IRREVERSIBLE"
	ParentIrreversible    ParentDisposition = "IRREVERSIBLE"
)

type ParentCut struct {
	Owner, Stream, HistoryGeneration string
	ThroughRevision                  uint64
	Digest                           [32]byte
	KeysetRevision                   uint64
	RequiredVersions                 []uint32
	Complete                         bool
}

// ParentObservation is an authenticated fact returned by the trusted User-only
// Auth client. It is not accepted directly by any transaction mutation helper.
type ParentObservation struct {
	AccountID              uuid.UUID
	OwnerHandle            string
	LifecycleRevision      uint64
	State                  ParentState
	Eligibility            ParentEligibility
	Disposition            ParentDisposition
	CycleID                uuid.UUID
	EraseAt                time.Time
	ObservedAt, ValidUntil time.Time
	Cut                    ParentCut
}

type ParentAuthority interface {
	ObserveParent(context.Context, uuid.UUID) (ParentObservation, error)
}
type ParentVerifier struct{ authority ParentAuthority }
type VerifiedParentProof struct {
	observation ParentObservation
	verified    bool
}

// NewParentVerifier is trusted startup dependency injection. Only the configured
// protected Auth client implements this authority in production; request data
// cannot construct a sealed proof or inject another verifier.
func NewParentVerifier(authority ParentAuthority) (*ParentVerifier, error) {
	if authority == nil {
		return nil, ErrIncomplete
	}
	return &ParentVerifier{authority: authority}, nil
}

// Verify performs the owner call before the caller opens its User transaction.
// Tx helpers consume the sealed result and never invoke this remote authority.
func (v *ParentVerifier) Verify(ctx context.Context, account uuid.UUID) (VerifiedParentProof, error) {
	if v == nil || v.authority == nil || account == uuid.Nil {
		return VerifiedParentProof{}, ErrIncomplete
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	observation, err := v.authority.ObserveParent(bounded, account)
	if err != nil || bounded.Err() != nil || !validParentObservation(observation, account, time.Now()) {
		return VerifiedParentProof{}, ErrIncomplete
	}
	observation.Cut.RequiredVersions = append([]uint32(nil), observation.Cut.RequiredVersions...)
	return VerifiedParentProof{observation: observation, verified: true}, nil
}

func validParentObservation(p ParentObservation, account uuid.UUID, now time.Time) bool {
	if p.AccountID != account || p.AccountID == uuid.Nil || p.OwnerHandle == "" || len(p.OwnerHandle) > 512 || strings.TrimSpace(p.OwnerHandle) != p.OwnerHandle || p.LifecycleRevision == 0 || p.LifecycleRevision > math.MaxInt64 || p.ObservedAt.IsZero() || p.ObservedAt.After(now) || !p.ValidUntil.After(p.ObservedAt) || !now.Before(p.ValidUntil) {
		return false
	}
	c := p.Cut
	if c.Owner != "auth" || c.Stream == "" || c.HistoryGeneration == "" || !c.Complete || c.ThroughRevision == 0 || c.ThroughRevision > math.MaxInt64 || c.KeysetRevision == 0 || c.KeysetRevision > math.MaxInt64 || c.Digest == ([32]byte{}) || len(c.RequiredVersions) == 0 {
		return false
	}
	var previous uint32
	for _, version := range c.RequiredVersions {
		if version <= previous {
			return false
		}
		previous = version
	}
	if p.Eligibility != ParentEligible && p.Eligibility != ParentIneligible {
		return false
	}
	switch p.State {
	case ParentActive:
		return p.Disposition == ParentNotIrreversible && p.CycleID == uuid.Nil && p.EraseAt.IsZero()
	case ParentRecoverable:
		return p.Disposition == ParentNotIrreversible && p.CycleID != uuid.Nil && !p.EraseAt.IsZero()
	case ParentTerminal:
		return p.Disposition == ParentIrreversible && p.Eligibility == ParentIneligible && p.CycleID != uuid.Nil && !p.EraseAt.IsZero() && !p.EraseAt.After(p.ObservedAt)
	}
	return false
}

func (p VerifiedParentProof) validAt(now time.Time) bool {
	return p.verified && validParentObservation(p.observation, p.observation.AccountID, now)
}
func (p VerifiedParentProof) validForBirth(now time.Time) bool {
	return p.validAt(now) && p.observation.State == ParentActive && p.observation.Eligibility == ParentEligible && p.observation.Disposition == ParentNotIrreversible
}
func (VerifiedParentProof) String() string   { return "User verified parent proof [redacted]" }
func (VerifiedParentProof) GoString() string { return "User verified parent proof [redacted]" }
