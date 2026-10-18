package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// LatchMatchSquadAbandonmentTx shares the caller's already locked abandonment
// transaction. It never performs provider IO or commits independently.
func LatchMatchSquadAbandonmentTx(ctx context.Context, tx pgx.Tx, matchID uuid.UUID, reason string, evidence [32]byte) error {
	return nil
}
