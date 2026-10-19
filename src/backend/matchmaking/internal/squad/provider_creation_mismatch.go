package squad

import (
	"context"
	"errors"

	"voice/backend/matchmaking/internal/store"
)

// Preserve positive contradictory response bytes, never a status error as an
// absence fact. The first witness remains private and permanently pages until
// an explicit reconciliation mechanism is qualified.
func (w *MatchSquadProviderWorker) recordCreationMismatch(ctx context.Context, operation store.MatchSquadProvisioningOperation, provider string, response []byte) error {
	hash := operation.ChatRequestHash
	if provider == "voice" {
		hash = operation.VoiceRequestHash
	}
	err := w.Store.RecordMatchSquadCreationMismatch(ctx, operation.MatchID, operation.OperationID, provider, hash, response)
	return errors.Join(errProviderContract, ErrProvisionOperatorAttention, store.ErrMatchSquadCreationMismatch, err)
}
