package squad

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"voice/backend/matchmaking/internal/store"
)

type ProvisioningPass struct {
	Outcomes []store.MatchSquadProvisioningOutcome
	Active   int
}

// ProvisionWithOutcome retains the established error contract while making
// abandoned settlement and successful activation distinct internal results.
func (w *MatchSquadProviderWorker) ProvisionWithOutcome(ctx context.Context, matchID uuid.UUID, profiles []uuid.UUID) (store.MatchSquadProvisioningOutcome, error) {
	_, _, err := w.provisionAttempt(ctx, matchID, profiles)
	return w.observeProvisionOutcome(ctx, matchID, err)
}

func (w *MatchSquadProviderWorker) Provision(ctx context.Context, matchID uuid.UUID, profiles []uuid.UUID) (string, string, error) {
	outcome, err := w.ProvisionWithOutcome(ctx, matchID, profiles)
	if err != nil {
		return "", "", err
	}
	if outcome.State != store.MatchSquadOutcomeActive {
		return "", "", store.ErrMatchSquadPending
	}
	return outcome.VoiceRoomID, outcome.ChatID, nil
}

func (w *MatchSquadProviderWorker) provisionOperationOutcome(ctx context.Context, operation store.MatchSquadProvisioningOperation) (store.MatchSquadProvisioningOutcome, error) {
	_, err := w.provisionOperation(ctx, operation)
	return w.observeProvisionOutcome(ctx, operation.MatchID, err)
}

func (w *MatchSquadProviderWorker) observeProvisionOutcome(ctx context.Context, matchID uuid.UUID, attemptErr error) (store.MatchSquadProvisioningOutcome, error) {
	if w == nil || w.Store == nil {
		return store.MatchSquadProvisioningOutcome{}, errors.Join(attemptErr, store.ErrSourcePartyUnavailable)
	}
	outcome, err := w.Store.GetMatchSquadProvisioningOutcome(ctx, matchID)
	if err != nil {
		return store.MatchSquadProvisioningOutcome{}, errors.Join(attemptErr, err)
	}
	if outcome.State == store.MatchSquadOutcomeContractMismatch {
		attemptErr = errors.Join(attemptErr, store.ErrMatchSquadCreationMismatch, ErrProvisionOperatorAttention)
	}
	if outcome.State != store.MatchSquadOutcomeActive && attemptErr == nil {
		attemptErr = store.ErrMatchSquadPending
	}
	return outcome, attemptErr
}

// RunProvisioningOnce is the existing runtime adapter; cleanup progress is never
// counted as successful provisioning. The typed pass retains each discovered cut.
func (w *MatchSquadProviderWorker) RunProvisioningOnce(ctx context.Context, limit int) (int, error) {
	pass, err := w.RunProvisioningWithOutcomesOnce(ctx, limit)
	return pass.Active, err
}
