package partysource

import (
	"context"
	"voice/backend/matchmaking/internal/store"
)

type ProjectionValidation struct {
	State   Validation
	Attempt *ValidatedProjectionAttempt
}
type ProjectionValidator interface {
	ValidateQueueProjection(context.Context, store.QueueProjectionCandidate) (ProjectionValidation, error)
}
type ValidatedProjectionAttempt struct{ hash [32]byte }

func (p *ValidatedProjectionAttempt) Matches(c store.QueueProjectionCandidate) bool {
	return p != nil && p.hash != [32]byte{} && p.hash == c.Hash()
}

func (r *Revalidator) ValidateQueueProjection(ctx context.Context, c store.QueueProjectionCandidate) (ProjectionValidation, error) {
	parties := c.Parties()
	if c.Hash() == [32]byte{} || len(c.Accounts()) == 0 || len(parties) != 1 {
		return ProjectionValidation{State: ValidationUnknown}, ErrUnknown
	}
	state, err := r.RevalidateCapturedParty(ctx, parties[0])
	if err != nil || state == ValidationUnknown {
		return ProjectionValidation{State: ValidationUnknown}, ErrUnknown
	}
	if state != ValidationCurrent {
		return ProjectionValidation{State: ValidationChanged}, nil
	}
	return ProjectionValidation{State: ValidationCurrent, Attempt: &ValidatedProjectionAttempt{hash: c.Hash()}}, nil
}
