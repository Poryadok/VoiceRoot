// Package automod evaluates already authenticated, ordered committed evidence.
// Persistence, identity, inbox deduplication and transaction ownership belong to
// the consuming store; no plaintext enters this package.
package automod

import (
	"errors"
	"time"
)

type Eligibility string

const (
	Eligible         Eligibility = "ELIGIBLE"
	NoText           Eligibility = "NO_TEXT"
	E2E              Eligibility = "E2E"
	ExpiredEvidence  Eligibility = "EXPIRED_EVIDENCE"
	EvidenceLifetime             = 30 * 24 * time.Hour
	RepeatWindow                 = 10 * time.Minute
	StrikeWindow                 = 30 * 24 * time.Hour
)

var (
	ErrSequence = errors.New("committed evidence sequence is not contiguous")
	ErrEvidence = errors.New("invalid committed evidence")
)

type Account struct {
	LastSequence  uint64
	LastSourceAt  time.Time
	MuteUntil     time.Time
	PostMuteCount uint32
	// Reload from effective machine sanctions under the account lock. An
	// authorized revocation can clear it; old inbox replay cannot reissue a ban.
	PermanentActive bool
}

type Sample struct {
	Sequence   uint64
	RecordedAt time.Time
}

type Pattern struct {
	Samples         []Sample
	LastHitAt       time.Time
	ConsumedThrough uint64
}

type Event struct {
	Sequence    uint64
	RecordedAt  time.Time
	Eligibility Eligibility
	LinkCount   uint32
}

type ActionKind string

const (
	Mute         ActionKind = "mute"
	PermanentBan ActionKind = "perm_ban"
)

type Action struct {
	Kind      ActionKind
	Sequence  uint64
	SourceAt  time.Time
	AppliedAt time.Time
	ExpiresAt time.Time
}

type Decision struct {
	Account    Account
	Pattern    Pattern
	Inspection Eligibility
	Actions    []Action
}

// Observe is pure: callers commit the returned state and actions together.
// A source gap/replay must be handled by the durable inbox before this function.
func Observe(account Account, pattern Pattern, event Event, appliedAt time.Time) (Decision, error) {
	if account.LastSequence == ^uint64(0) || event.Sequence != account.LastSequence+1 {
		return Decision{}, ErrSequence
	}
	if event.RecordedAt.IsZero() || appliedAt.IsZero() || event.RecordedAt.Before(account.LastSourceAt) || event.RecordedAt.After(appliedAt.Add(5*time.Second)) {
		return Decision{}, ErrEvidence
	}
	switch event.Eligibility {
	case Eligible:
	case NoText, E2E:
		if event.LinkCount != 0 {
			return Decision{}, ErrEvidence
		}
	case ExpiredEvidence:
		if appliedAt.Before(event.RecordedAt.Add(EvidenceLifetime)) || event.LinkCount != 0 {
			return Decision{}, ErrEvidence
		}
	default:
		return Decision{}, ErrEvidence
	}
	account.LastSequence = event.Sequence
	account.LastSourceAt = event.RecordedAt
	if !account.MuteUntil.IsZero() && !event.RecordedAt.Before(account.MuteUntil) && account.PostMuteCount < 10 {
		account.PostMuteCount++
	}
	inspection := event.Eligibility
	if !appliedAt.Before(event.RecordedAt.Add(EvidenceLifetime)) {
		inspection = ExpiredEvidence
	}
	decision := Decision{Account: account, Pattern: pattern, Inspection: inspection}
	if inspection != Eligible {
		return decision, nil
	}
	if event.Sequence <= pattern.ConsumedThrough {
		return Decision{}, ErrEvidence
	}
	// Allocate a new slice; a failed caller transaction must not mutate its
	// previously loaded snapshot through a shared backing array.
	samples := make([]Sample, 0, len(pattern.Samples)+1)
	cutoff := event.RecordedAt.Add(-RepeatWindow)
	for _, sample := range pattern.Samples {
		if sample.Sequence > pattern.ConsumedThrough && !sample.RecordedAt.Before(cutoff) {
			samples = append(samples, sample)
		}
	}
	samples = append(samples, Sample{Sequence: event.Sequence, RecordedAt: event.RecordedAt})
	pattern.Samples = samples
	if event.LinkCount < 3 && len(samples) < 5 {
		decision.Pattern = pattern
		return decision, nil
	}
	second := !pattern.LastHitAt.IsZero() && !event.RecordedAt.After(pattern.LastHitAt.Add(StrikeWindow))
	pattern.LastHitAt = event.RecordedAt
	pattern.ConsumedThrough = event.Sequence
	pattern.Samples = nil
	action := Action{Sequence: event.Sequence, SourceAt: event.RecordedAt, AppliedAt: appliedAt}
	if second {
		if !account.PermanentActive {
			action.Kind = PermanentBan
			account.PermanentActive = true
			decision.Actions = []Action{action}
		}
	} else {
		action.Kind = Mute
		action.ExpiresAt = appliedAt.Add(time.Hour)
		if action.ExpiresAt.After(account.MuteUntil) {
			account.MuteUntil = action.ExpiresAt
			account.PostMuteCount = 0
		}
		decision.Actions = []Action{action}
	}
	decision.Account, decision.Pattern = account, pattern
	return decision, nil
}
