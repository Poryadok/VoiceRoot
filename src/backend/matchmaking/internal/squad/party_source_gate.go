package squad

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

var ErrPartyProvisionUnknown = errors.New("provision party authority is unknown")
var ErrPartyProvisionChanged = errors.New("provision party changed and was reset")

// PartySourceGate is shared by request and background provider coordinators.
// It performs external authority IO outside SQL locks. The returned candidate
// still needs VerifyProvisionCandidateTx under canonical locks at each immediate
// intent/dispatch/activation boundary; it is never a reusable authorization cache.
type PartySourceGate struct {
	Store     *store.MatchStore
	Validator partysource.ProvisionValidator
}

func (g *PartySourceGate) ValidateProvision(ctx context.Context, matchID uuid.UUID) (store.ProvisionCandidate, error) {
	if g == nil || g.Store == nil || g.Validator == nil || matchID == uuid.Nil {
		return store.ProvisionCandidate{}, ErrPartyProvisionUnknown
	}
	candidate, err := g.Store.CaptureProvisionCandidate(ctx, matchID)
	if err != nil {
		return store.ProvisionCandidate{}, errors.Join(ErrPartyProvisionUnknown, err)
	}
	result, err := g.Validator.ValidateCapturedParties(ctx, candidate)
	if err != nil {
		// An error dominates any partial changed result: no cancellation can be
		// inferred while another source remains unavailable or ambiguous.
		return store.ProvisionCandidate{}, errors.Join(ErrPartyProvisionUnknown, err)
	}
	if result.Attempt != nil {
		if len(result.ChangedPartyIDs) != 0 || !result.Attempt.Matches(candidate) {
			return store.ProvisionCandidate{}, ErrPartyProvisionUnknown
		}
		return candidate, nil
	}
	if len(result.ChangedPartyIDs) == 0 {
		return store.ProvisionCandidate{}, ErrPartyProvisionUnknown
	}
	parties := candidate.Parties()
	allowed := make(map[uuid.UUID]bool, len(parties))
	for _, party := range parties {
		allowed[party.ID] = true
	}
	for _, id := range result.ChangedPartyIDs {
		if id == uuid.Nil || !allowed[id] {
			return store.ProvisionCandidate{}, ErrPartyProvisionUnknown
		}
	}
	if _, err := g.Store.ResetVoiceParties(ctx, result.ChangedPartyIDs); err != nil {
		return store.ProvisionCandidate{}, errors.Join(ErrPartyProvisionUnknown, err)
	}
	return store.ProvisionCandidate{}, ErrPartyProvisionChanged
}
