package partysource

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/store"
)

// ValidatedProposalAttempt observes one exact pre-reservation local cut. The
// transition must verify that cut under account/party/session locks; this proof
// is neither a persisted grant nor authority for another capture.
type ValidatedProposalAttempt struct{ candidateHash [32]byte }

func (a *ValidatedProposalAttempt) Matches(c store.ProposalCandidate) bool {
	return a != nil && a.candidateHash != [32]byte{} && a.candidateHash == c.Hash()
}

type ProposalValidation struct {
	Attempt         *ValidatedProposalAttempt
	ChangedPartyIDs []uuid.UUID
}
type ProposalValidator interface {
	ValidateProposalCandidate(context.Context, store.ProposalCandidate) (ProposalValidation, error)
}

func (r *Revalidator) ValidateProposalCandidate(ctx context.Context, candidate store.ProposalCandidate) (ProposalValidation, error) {
	if r == nil || r.Snapshots == nil || candidate.Hash() == [32]byte{} || len(candidate.Accounts()) == 0 {
		return ProposalValidation{}, ErrUnknown
	}
	parties := candidate.Parties()
	if len(parties) == 0 {
		return ProposalValidation{}, ErrUnknown
	}
	var changed []uuid.UUID
	for _, party := range parties {
		result, err := r.RevalidateCapturedParty(ctx, party)
		if err != nil || result == ValidationUnknown {
			return ProposalValidation{}, errors.Join(ErrUnknown, err)
		}
		if result == ValidationChanged {
			changed = append(changed, party.ID)
		}
	}
	if len(changed) != 0 {
		return ProposalValidation{ChangedPartyIDs: changed}, nil
	}
	return ProposalValidation{Attempt: &ValidatedProposalAttempt{candidateHash: candidate.Hash()}}, nil
}
