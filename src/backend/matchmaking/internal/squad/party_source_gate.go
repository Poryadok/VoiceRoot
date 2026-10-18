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
	return store.ProvisionCandidate{}, ErrPartyProvisionUnknown
}
