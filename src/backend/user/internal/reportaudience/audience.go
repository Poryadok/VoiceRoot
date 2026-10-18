// Package reportaudience assembles complete, short-lived User/Auth observations.
// It never substitutes an environment value or a partial account classification.
package reportaudience

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
)

const MetricVersion = "POTENTIAL_PUBLIC_PROFILE_ACCOUNTS_V1"
const ObservationLifetime = 30 * time.Second
const BatchSize = 1000

type State uint8

const (
	Unknown State = iota
	Live
	Deleted
	Missing
	EmptyAudience
)

var ErrUnavailable = errors.New("complete report audience authority unavailable")

type Binding struct {
	ProfileID, AccountID uuid.UUID
	Revision             uint64
	State                State
}

type Snapshot struct {
	Binding
	Candidates []uuid.UUID
}

type Account struct {
	ID    uuid.UUID
	State State
}
type Classification struct {
	EvaluationID, SnapshotID          uuid.UUID
	Accounts                          []Account
	StartedAt, CompletedAt, ExpiresAt time.Time
}

type Source interface {
	Snapshot(context.Context, uuid.UUID) (Snapshot, error)
	Recheck(context.Context, Binding) error
}
type Classifier interface {
	Classify(context.Context, uuid.UUID, []uuid.UUID) (Classification, error)
}
type Service struct {
	Source     Source
	Classifier Classifier
	Now        func() time.Time
}
type Result struct {
	Binding
	EvaluationID, SnapshotID          uuid.UUID
	Metric                            string
	Audience                          int64
	StartedAt, CompletedAt, ExpiresAt time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

// Resolve does no remote work while the source snapshot transaction is open.
// Expected binds a retry to an already observed target; it cannot authorize a changed owner.
func (s Service) Resolve(ctx context.Context, evaluation, profile uuid.UUID, expected *Binding) (Result, error) {
	if evaluation == uuid.Nil || profile == uuid.Nil || s.Source == nil || s.Classifier == nil {
		return Result{}, ErrUnavailable
	}
	started := s.now()
	ctx, cancel := context.WithTimeout(ctx, ObservationLifetime)
	defer cancel()
	snapshot, err := s.Source.Snapshot(ctx, profile)
	if err != nil || snapshot.ProfileID != profile {
		return Result{}, ErrUnavailable
	}
	if snapshot.State != Missing && (snapshot.AccountID == uuid.Nil || snapshot.Revision == 0) {
		return Result{}, ErrUnavailable
	}
	if expected != nil && (snapshot.AccountID != expected.AccountID || snapshot.Revision != expected.Revision) {
		return Result{}, ErrUnavailable
	}
	result := Result{Binding: snapshot.Binding, EvaluationID: evaluation, SnapshotID: uuid.New(), Metric: MetricVersion, StartedAt: started, ExpiresAt: started.Add(ObservationLifetime)}
	if snapshot.State == Missing || snapshot.State == Deleted {
		result.CompletedAt = s.now()
		if !result.CompletedAt.Before(result.ExpiresAt) {
			return Result{}, ErrUnavailable
		}
		return result, nil
	}
	if snapshot.State != Live {
		return Result{}, ErrUnavailable
	}
	seen := make(map[uuid.UUID]struct{}, len(snapshot.Candidates))
	for _, id := range snapshot.Candidates {
		if id == uuid.Nil || id == snapshot.AccountID {
			return Result{}, ErrUnavailable
		}
		if _, exists := seen[id]; exists {
			return Result{}, ErrUnavailable
		}
		seen[id] = struct{}{}
	}
	for offset := 0; offset < len(snapshot.Candidates); offset += BatchSize {
		end := min(offset+BatchSize, len(snapshot.Candidates))
		batch := snapshot.Candidates[offset:end]
		classification, classifyErr := s.Classifier.Classify(ctx, evaluation, batch)
		observed := s.now()
		if classifyErr != nil || classification.EvaluationID != evaluation || classification.SnapshotID == uuid.Nil ||
			classification.StartedAt.IsZero() || classification.CompletedAt.Before(classification.StartedAt) || classification.CompletedAt.After(observed) ||
			!classification.ExpiresAt.After(observed) || classification.ExpiresAt.After(classification.StartedAt.Add(ObservationLifetime)) ||
			len(classification.Accounts) != len(batch) {
			return Result{}, ErrUnavailable
		}
		if classification.StartedAt.Before(result.StartedAt) {
			result.StartedAt = classification.StartedAt
		}
		if classification.ExpiresAt.Before(result.ExpiresAt) {
			result.ExpiresAt = classification.ExpiresAt
		}
		wanted := make(map[uuid.UUID]struct{}, len(batch))
		for _, id := range batch {
			wanted[id] = struct{}{}
		}
		for _, account := range classification.Accounts {
			if _, exists := wanted[account.ID]; !exists {
				return Result{}, ErrUnavailable
			}
			delete(wanted, account.ID)
			switch account.State {
			case Live:
				result.Audience++
			case Deleted, Missing:
			default:
				return Result{}, ErrUnavailable
			}
		}
	}
	if err := s.Source.Recheck(ctx, snapshot.Binding); err != nil {
		return Result{}, ErrUnavailable
	}
	result.CompletedAt = s.now()
	if ctx.Err() != nil || !result.CompletedAt.Before(result.ExpiresAt) || result.CompletedAt.Sub(result.StartedAt) >= ObservationLifetime {
		return Result{}, ErrUnavailable
	}
	if result.Audience == 0 {
		result.State = EmptyAudience
	}
	return result, nil
}
