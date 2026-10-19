package matcher

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/store"
)

var (
	ErrPartyProposalUnknown = errors.New("party proposal authority unavailable")
	ErrPartyProposalChanged = errors.New("party proposal source changed")
)

func (w *Worker) reserveGroup(ctx context.Context, params store.CreateProposalParams) (store.CreateProposalResult, error) {
	if w.ProposalValidator == nil {
		// Preserve the pre-cut legacy path. The storage API independently refuses
		// every typed source; absence of this field cannot enable native parties.
		return w.Matches.CreateProposal(ctx, params)
	}
	candidate, err := w.Matches.CaptureProposalCandidate(ctx, params)
	if err != nil {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	validation, err := w.ProposalValidator.ValidateProposalCandidate(ctx, candidate)
	if err != nil {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	if len(validation.ChangedPartyIDs) != 0 {
		if validation.Attempt != nil {
			return store.CreateProposalResult{}, ErrPartyProposalUnknown
		}
		return w.resetChangedProposal(ctx, candidate, validation.ChangedPartyIDs)
	}
	if !validation.Attempt.Matches(candidate) {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	result, err := w.Matches.CreateValidatedProposal(ctx, candidate, validation.Attempt)
	if err == nil {
		return result, nil
	}
	var denial *store.PlatformBanDenial
	if !errors.As(err, &denial) || denial == nil {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	// Only typed private evidence from the fresh local ban read may select a
	// whole-party reset. Generic denial/transport errors have no such authority.
	denied := denial.Accounts()
	if len(denied) == 0 {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	allowed := make(map[uuid.UUID]bool)
	for _, account := range candidate.Accounts() {
		allowed[account] = true
	}
	accountSet := make(map[uuid.UUID]bool, len(denied))
	for _, account := range denied {
		if !allowed[account] || accountSet[account] {
			return store.CreateProposalResult{}, ErrPartyProposalUnknown
		}
		accountSet[account] = true
	}
	var changed []uuid.UUID
	for _, party := range candidate.Parties() {
		for _, member := range party.MemberAuthorities {
			if accountSet[member.AccountID] {
				changed = append(changed, party.ID)
				break
			}
		}
	}
	return w.resetChangedProposal(ctx, candidate, changed)
}

func (w *Worker) resetChangedProposal(ctx context.Context, candidate store.ProposalCandidate, changed []uuid.UUID) (store.CreateProposalResult, error) {
	allowed := make(map[uuid.UUID]bool)
	for _, party := range candidate.Parties() {
		allowed[party.ID] = true
	}
	seen := make(map[uuid.UUID]bool, len(changed))
	if len(changed) == 0 {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	for _, id := range changed {
		if !allowed[id] || seen[id] {
			return store.CreateProposalResult{}, ErrPartyProposalUnknown
		}
		seen[id] = true
	}
	if _, err := w.Matches.ResetVoiceParties(ctx, changed); err != nil {
		return store.CreateProposalResult{}, ErrPartyProposalUnknown
	}
	return store.CreateProposalResult{}, ErrPartyProposalChanged
}
