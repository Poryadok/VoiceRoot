package partysearch

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

type ProjectionLifetime interface {
	EnterForward(context.Context, uuid.UUID, [32]byte) (func(), error)
}
type ProjectionResetter interface {
	ResetVoiceParty(context.Context, uuid.UUID) (store.VoiceRosterResetResult, error)
}

func (p *Projector) authorizeNativeProjection(ctx context.Context, cut store.PartyQueueProjection) (func(), error) {
	if p.Validator == nil || p.ForwardLifetime == nil || p.Resetter == nil {
		return nil, partysource.ErrUnknown
	}
	candidate, err := p.Store.CaptureQueueProjection(ctx, cut)
	if err != nil {
		return nil, err
	}
	validation, err := p.Validator.ValidateQueueProjection(ctx, candidate)
	if err != nil || validation.State == partysource.ValidationUnknown {
		return nil, partysource.ErrUnknown
	}
	reset := func() error {
		parties := candidate.Parties()
		if len(parties) != 1 {
			return partysource.ErrUnknown
		}
		_, err := p.Resetter.ResetVoiceParty(ctx, parties[0].ID)
		if err != nil {
			return err
		}
		return store.ErrPartyQueueProjectionChanged
	}
	if validation.State == partysource.ValidationChanged {
		return nil, reset()
	}
	if validation.State != partysource.ValidationCurrent || validation.Attempt == nil || !validation.Attempt.Matches(candidate) {
		return nil, partysource.ErrUnknown
	}
	release, err := p.ForwardLifetime.EnterForward(ctx, uuid.New(), candidate.Hash())
	if err != nil || release == nil {
		if release != nil {
			release()
		}
		return nil, partysource.ErrUnknown
	}
	err = p.Store.AuthorizeQueueProjection(ctx, candidate, validation.Attempt)
	if err != nil {
		release()
		var denial *store.PlatformBanDenial
		if errors.As(err, &denial) {
			allowed := map[uuid.UUID]bool{}
			for _, id := range candidate.Accounts() {
				allowed[id] = true
			}
			denied := denial.Accounts()
			if len(denied) == 0 {
				return nil, partysource.ErrUnknown
			}
			for _, id := range denied {
				if !allowed[id] {
					return nil, partysource.ErrUnknown
				}
			}
			return nil, reset()
		}
		return nil, err
	}
	return release, nil
}
