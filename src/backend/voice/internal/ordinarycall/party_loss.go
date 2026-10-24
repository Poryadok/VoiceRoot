package ordinarycall

import (
	"context"
	"errors"
	"github.com/google/uuid"
)

var ErrPartySourceChanged = errors.New("ordinary party source changed")

// PartyEligibilityLoss is an exact native witness captured before fresh Auth IO.
// It cannot remove a replacement membership of the same account/profile.
type PartyEligibilityLoss struct {
	RoomID, Incarnation, AccountID, ProfileID, OperationID, Generation uuid.UUID
	SessionEpoch                                                       int64
}

func (s *PostgresStore) ApplyPartyEligibilityLoss(ctx context.Context, loss PartyEligibilityLoss) error {
	if loss.RoomID == uuid.Nil || loss.Incarnation == uuid.Nil || loss.AccountID == uuid.Nil || loss.ProfileID == uuid.Nil || loss.OperationID == uuid.Nil || loss.Generation == uuid.Nil || loss.SessionEpoch <= 0 {
		return ErrInvalidIntent
	}
	_, err := s.Mutate(ctx, loss.RoomID, func(r *Room) error {
		m, ok := r.Members[loss.ProfileID.String()]
		if !ok || r.Intent.OperationID != loss.Incarnation || r.Stage != StageActive || m.State != MemberJoined || m.AccountID != loss.AccountID || m.Generation != loss.Generation || m.FenceOperationID != loss.OperationID || m.SessionEpoch != loss.SessionEpoch {
			return ErrPartySourceChanged
		}
		return r.DecideLeave(loss.ProfileID, r.TransactionTime)
	})
	if errors.Is(err, ErrNotFound) {
		return ErrPartySourceChanged
	}
	return err
}
