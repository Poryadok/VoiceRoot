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
	candidate, _, err := g.ValidateAttempt(ctx, matchID)
	return candidate, err
}

// ValidateAttempt returns the actual private proof issued by the source
// validator. The coordinator consumes it at exactly one store transition.
func (g *PartySourceGate) ValidateAttempt(ctx context.Context, matchID uuid.UUID) (store.ProvisionCandidate, *partysource.ValidatedAttempt, error) {
	if g == nil || g.Store == nil || g.Validator == nil || matchID == uuid.Nil {
		return store.ProvisionCandidate{}, nil, ErrPartyProvisionUnknown
	}
	candidate, err := g.Store.CaptureProvisionCandidate(ctx, matchID)
	if err != nil {
		return store.ProvisionCandidate{}, nil, errors.Join(ErrPartyProvisionUnknown, err)
	}
	result, err := g.Validator.ValidateCapturedParties(ctx, candidate)
	if err != nil {
		// An error dominates any partial changed result: no cancellation can be
		// inferred while another source remains unavailable or ambiguous.
		return store.ProvisionCandidate{}, nil, errors.Join(ErrPartyProvisionUnknown, err)
	}
	if result.Attempt != nil {
		if len(result.ChangedPartyIDs) != 0 || !result.Attempt.Matches(candidate) {
			return store.ProvisionCandidate{}, nil, ErrPartyProvisionUnknown
		}
		return candidate, result.Attempt, nil
	}
	if len(result.ChangedPartyIDs) == 0 {
		return store.ProvisionCandidate{}, nil, ErrPartyProvisionUnknown
	}
	parties := candidate.Parties()
	allowed := make(map[uuid.UUID]bool, len(parties))
	for _, party := range parties {
		allowed[party.ID] = true
	}
	for _, id := range result.ChangedPartyIDs {
		if id == uuid.Nil || !allowed[id] {
			return store.ProvisionCandidate{}, nil, ErrPartyProvisionUnknown
		}
	}
	if _, err := g.Store.ResetVoiceParties(ctx, result.ChangedPartyIDs); err != nil {
		return store.ProvisionCandidate{}, nil, errors.Join(ErrPartyProvisionUnknown, err)
	}
	return store.ProvisionCandidate{}, nil, ErrPartyProvisionChanged
}

// ResetDeniedParties consumes only the exact local guarded ban evidence. A
// generic denial or any account outside the captured cut remains unknown.
func (g *PartySourceGate) ResetDeniedParties(ctx context.Context, candidate store.ProvisionCandidate, denial error) error {
	var exact *store.PlatformBanDenial
	if g == nil || g.Store == nil || !errors.As(denial, &exact) || exact == nil {
		return ErrPartyProvisionUnknown
	}
	accounts := exact.Accounts()
	if len(accounts) == 0 {
		return ErrPartyProvisionUnknown
	}
	allowed := map[uuid.UUID]bool{}
	for _, account := range candidate.Accounts() {
		allowed[account] = true
	}
	denied := map[uuid.UUID]bool{}
	for _, account := range accounts {
		if account == uuid.Nil || !allowed[account] {
			return ErrPartyProvisionUnknown
		}
		denied[account] = true
	}
	var affected []uuid.UUID
	for _, party := range candidate.Parties() {
		for _, member := range party.MemberAuthorities {
			if denied[member.AccountID] {
				affected = append(affected, party.ID)
				break
			}
		}
	}
	if len(affected) == 0 {
		return ErrPartyProvisionUnknown
	}
	if _, err := g.Store.ResetVoiceParties(ctx, affected); err != nil {
		return errors.Join(ErrPartyProvisionUnknown, err)
	}
	return ErrPartyProvisionChanged
}
