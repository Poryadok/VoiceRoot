package partysnapshot

import (
	"context"
	"errors"
	"unicode/utf8"

	"github.com/google/uuid"

	"voice/backend/voice/internal/ordinarycall"
	"voice/backend/voice/internal/spacemedia"
)

type OrdinaryLossWriter interface {
	ApplyPartyEligibilityLoss(context.Context, ordinarycall.PartyEligibilityLoss) error
}
type SpaceLossWriter interface {
	ApplyPartyEligibilityLoss(context.Context, spacemedia.PartyEligibilityLoss) error
}

// NativeNormalizer delegates definitive eligibility loss to the owning writer.
// It never repairs ownership or invents reconnect support in a native source.
type NativeNormalizer struct {
	Ordinary OrdinaryLossWriter
	Space    SpaceLossWriter
}

func (n *NativeNormalizer) ApplyEligibilityLoss(ctx context.Context, loss LossTarget) error {
	m := loss.Member
	if n == nil || loss.RoomID == uuid.Nil || loss.Incarnation == uuid.Nil || m.AccountID == uuid.Nil || m.ProfileID == uuid.Nil || m.OperationID == uuid.Nil || m.SessionEpoch <= 0 || m.Generation == "" || !utf8.ValidString(m.Generation) || m.State != Joined || loss.Reason != "SESSION_REVOKED" {
		return ErrUnavailable
	}
	var err error
	switch loss.SourceKind {
	case "ORDINARY":
		generation, parseErr := uuid.Parse(m.Generation)
		if parseErr != nil || generation == uuid.Nil || generation.String() != m.Generation || n.Ordinary == nil {
			return ErrUnavailable
		}
		err = n.Ordinary.ApplyPartyEligibilityLoss(ctx, ordinarycall.PartyEligibilityLoss{RoomID: loss.RoomID, Incarnation: loss.Incarnation, AccountID: m.AccountID, ProfileID: m.ProfileID, OperationID: m.OperationID, Generation: generation, SessionEpoch: m.SessionEpoch})
	case "SPACE":
		if n.Space == nil {
			return ErrUnavailable
		}
		err = n.Space.ApplyPartyEligibilityLoss(ctx, spacemedia.PartyEligibilityLoss{RoomID: loss.RoomID, Incarnation: loss.Incarnation, AccountID: m.AccountID, ProfileID: m.ProfileID, OperationID: m.OperationID, Generation: m.Generation, SessionEpoch: m.SessionEpoch})
	default:
		return ErrUnavailable
	}
	if errors.Is(err, ordinarycall.ErrPartySourceChanged) || errors.Is(err, spacemedia.ErrPartySourceChanged) {
		return ErrSourceChanged
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}
