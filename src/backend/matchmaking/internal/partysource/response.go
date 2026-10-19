package partysource

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/store"
)

type ValidatedResponseAttempt struct{ candidateHash [32]byte }

func (a *ValidatedResponseAttempt) Matches(c store.ResponseCandidate) bool {
	return a != nil && a.candidateHash != ([32]byte{}) && a.candidateHash == c.Hash()
}

type ResponseValidation struct {
	Attempt         *ValidatedResponseAttempt
	ChangedPartyIDs []uuid.UUID
}
type ResponseValidator interface {
	ValidateResponseCandidate(context.Context, store.ResponseCandidate) (ResponseValidation, error)
}

func (r *Revalidator) ValidateResponseCandidate(ctx context.Context, c store.ResponseCandidate) (ResponseValidation, error) {
	if r == nil || r.Snapshots == nil || c.Hash() == ([32]byte{}) || len(c.Accounts()) == 0 {
		return ResponseValidation{}, ErrUnknown
	}
	parties := c.Parties()
	if len(parties) == 0 {
		return ResponseValidation{}, ErrUnknown
	}
	var changed []uuid.UUID
	for _, party := range parties {
		state, err := r.RevalidateCapturedParty(ctx, party)
		if err != nil || state == ValidationUnknown {
			return ResponseValidation{}, errors.Join(ErrUnknown, err)
		}
		if state == ValidationChanged {
			changed = append(changed, party.ID)
		}
	}
	if len(changed) > 0 {
		return ResponseValidation{ChangedPartyIDs: changed}, nil
	}
	return ResponseValidation{Attempt: &ValidatedResponseAttempt{candidateHash: c.Hash()}}, nil
}
