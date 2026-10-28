// Package terminalrouting implements the local terminal-routing consumer of
// A4-ACCOUNT-LIFECYCLE-PRODUCER-v1. It does not authenticate producer transport.
package terminalrouting

import (
	"errors"
	"time"
	"unicode/utf8"
)

var (
	ErrAuthorityInvalid       = errors.New("terminal routing authority invalid")
	ErrAuthorityConflict      = errors.New("terminal routing authority conflict")
	ErrReconciliationRequired = errors.New("terminal routing reconciliation required")
	ErrRoutingRetired         = errors.New("terminal routing permanently retired")
)

type State string

const (
	Active          State = "ACTIVE"
	DeleteScheduled State = "DELETE_SCHEDULED"
	AccountPurged   State = "ACCOUNT_PURGED"
)

type DecisionKind string

const Restored DecisionKind = "RESTORED"
const Purged DecisionKind = "ACCOUNT_PURGED"

// Schedule is an immutable, already verified Auth schedule, not an event age.
type Schedule struct {
	FenceID, CycleID     string
	Revision             uint64
	ScheduledAt, PurgeAt time.Time
}

type Decision struct {
	Kind     DecisionKind
	Revision uint64
	At       time.Time
}

type Current struct {
	Revision uint64
	State    State
	CycleID  string
}

// Observation joins the queried cycle's immutable decision with independent
// current account authority. Callers must verify their protected binding first.
type Observation struct {
	Schedule Schedule
	Decision *Decision
	Current  Current
}

// Lifecycle contains only opaque lifecycle metadata. Raw profile manifests and
// terminal routing belong to the durable store, which must erase them atomically
// with RoutingErased using its own database clock.
type Lifecycle struct {
	Schedule      Schedule
	Decision      *Decision
	Current       Current
	RoutingErased bool
	reconciled    bool
}

func New(schedule Schedule) (Lifecycle, error) {
	if !validSchedule(schedule) {
		return Lifecycle{}, ErrAuthorityInvalid
	}
	return Lifecycle{Schedule: schedule, Current: Current{
		Revision: schedule.Revision, State: DeleteScheduled, CycleID: schedule.CycleID,
	}}, nil
}

func validSchedule(s Schedule) bool {
	return s.FenceID != "" && s.CycleID != "" && utf8.ValidString(s.FenceID) && utf8.ValidString(s.CycleID) &&
		s.Revision > 0 && !s.ScheduledAt.IsZero() && s.PurgeAt.After(s.ScheduledAt)
}

func sameSchedule(a, b Schedule) bool {
	return a.FenceID == b.FenceID && a.CycleID == b.CycleID && a.Revision == b.Revision &&
		a.ScheduledAt.Equal(b.ScheduledAt) && a.PurgeAt.Equal(b.PurgeAt)
}

// Advance never infers Auth ACCOUNT_PURGED. A previously verified exact-cycle
// restore cancels pending erasure, but cannot reverse an already erased outcome.
func (l Lifecycle) Advance(now time.Time) Lifecycle {
	if !l.RoutingErased && !now.Before(l.Schedule.PurgeAt) &&
		(l.Decision == nil || l.Decision.Kind != Restored) {
		l.RoutingErased = true
	}
	return l
}

func validCurrent(c Current) bool {
	if c.Revision == 0 {
		return false
	}
	switch c.State {
	case Active:
		return c.CycleID == ""
	case DeleteScheduled, AccountPurged:
		return c.CycleID != "" && utf8.ValidString(c.CycleID)
	default:
		return false
	}
}

func validDecision(s Schedule, d Decision) bool {
	if d.Revision <= s.Revision || d.At.Before(s.ScheduledAt) {
		return false
	}
	switch d.Kind {
	case Restored:
		return d.At.Before(s.PurgeAt)
	case Purged:
		return !d.At.Before(s.PurgeAt)
	default:
		return false
	}
}

func (l Lifecycle) Reconcile(o Observation, now time.Time) (Lifecycle, error) {
	next := l.Advance(now)
	if !sameSchedule(l.Schedule, o.Schedule) {
		return next, ErrAuthorityConflict
	}
	if !validCurrent(o.Current) || o.Current.Revision < l.Schedule.Revision {
		return next, ErrAuthorityInvalid
	}
	if o.Decision != nil {
		if !validDecision(l.Schedule, *o.Decision) || o.Current.Revision < o.Decision.Revision {
			return next, ErrAuthorityInvalid
		}
		if l.Decision != nil && (l.Decision.Kind != o.Decision.Kind || l.Decision.Revision != o.Decision.Revision || !l.Decision.At.Equal(o.Decision.At)) {
			return next, ErrAuthorityConflict
		}
		d := *o.Decision
		next.Decision = &d
	}
	if o.Current.Revision == l.Current.Revision && o.Current != l.Current {
		return next, ErrAuthorityConflict
	}
	if o.Current.Revision > l.Current.Revision {
		next.Current = o.Current
	}
	next.reconciled = true
	return next, nil
}

func (l Lifecycle) CanProjectOldRouting(now time.Time) bool {
	return !l.Advance(now).RoutingErased
}

// FreshAdmission requires the separately verified current profile eligibility.
// This method never authorizes reconstructing an old operation or payload.
func (l Lifecycle) FreshAdmission(currentRevision uint64, currentProfileEligible bool) bool {
	return l.reconciled && l.Decision != nil && l.Decision.Kind == Restored &&
		l.Current.State == Active && l.Current.Revision == currentRevision && currentProfileEligible
}

// RestoreForServing is the local bootstrap rule, not a backup tool or an Auth
// lookup. Missing protected reconciliation keeps admission closed, while passed
// local deadlines still apply. It intentionally cannot rehydrate raw routing.
func RestoreForServing(snapshot Lifecycle, observation *Observation, now time.Time) (Lifecycle, error) {
	snapshot.reconciled = false
	snapshot = snapshot.Advance(now)
	if observation == nil {
		return snapshot, ErrReconciliationRequired
	}
	return snapshot.Reconcile(*observation, now)
}
