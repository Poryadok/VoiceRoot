package spacemedia

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"voice/backend/voice/internal/store"
)

var ErrPartySourceChanged = errors.New("space party source changed")

type PartyEligibilityLoss struct {
	RoomID, Incarnation, AccountID, ProfileID, OperationID uuid.UUID
	Generation                                             string
	SessionEpoch                                           int64
}

// ApplyPartyEligibilityLoss normalizes a definitive Auth loss against the exact
// captured native member. Physical media cleanup remains the existing recovery.
func (s *PostgresAdmissionStore) ApplyPartyEligibilityLoss(ctx context.Context, loss PartyEligibilityLoss) error {
	if loss.RoomID == uuid.Nil || loss.Incarnation == uuid.Nil || loss.AccountID == uuid.Nil || loss.ProfileID == uuid.Nil || loss.OperationID == uuid.Nil || loss.Generation == "" || loss.SessionEpoch <= 0 {
		return ErrAdmissionConflict
	}
	return s.beginRoomLeave(ctx, loss.OperationID, loss.Generation, &loss, func(tx pgx.Tx) error {
		var target store.SpaceAdmissionRevocation
		return readRevocationTarget(ctx, tx, loss.OperationID, loss.Generation, &target)
	})
}
