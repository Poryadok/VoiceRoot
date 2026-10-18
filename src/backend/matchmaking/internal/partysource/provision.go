package partysource

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/store"
)

// ValidatedAttempt is an observation for one immediate local transition, never
// a reusable authorization cache. Its matching candidate must be verified under
// canonical store locks before the forward transition commits.
type ValidatedAttempt struct{ candidateHash [32]byte }

func (a *ValidatedAttempt) Matches(c store.ProvisionCandidate) bool {
	return a != nil && a.candidateHash != [32]byte{} && a.candidateHash == c.Hash()
}
func (a *ValidatedAttempt) Hash() [32]byte {
	if a == nil {
		return [32]byte{}
	}
	return a.candidateHash
}

type ProvisionValidation struct {
	Attempt         *ValidatedAttempt
	ChangedPartyIDs []uuid.UUID
}
type ProvisionValidator interface {
	ValidateCapturedParties(context.Context, store.ProvisionCandidate) (ProvisionValidation, error)
}

func (r *Revalidator) ValidateCapturedParties(ctx context.Context, c store.ProvisionCandidate) (ProvisionValidation, error) {
	if r == nil || r.Snapshots == nil || c.MatchID == uuid.Nil || c.Hash() == [32]byte{} {
		return ProvisionValidation{}, ErrUnknown
	}
	parties := c.Parties()
	if len(parties) == 0 {
		return ProvisionValidation{}, ErrUnknown
	}
	var changed []uuid.UUID
	for _, p := range parties {
		result, err := r.RevalidateCapturedParty(ctx, p)
		if err != nil || result == ValidationUnknown {
			return ProvisionValidation{}, errors.Join(ErrUnknown, err)
		}
		if result == ValidationChanged {
			changed = append(changed, p.ID)
		}
	}
	if len(changed) > 0 {
		return ProvisionValidation{ChangedPartyIDs: changed}, nil
	}
	return ProvisionValidation{Attempt: &ValidatedAttempt{candidateHash: c.Hash()}}, nil
}
