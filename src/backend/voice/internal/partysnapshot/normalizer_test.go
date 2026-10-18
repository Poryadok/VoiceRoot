package partysnapshot

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/voice/internal/ordinarycall"
	"voice/backend/voice/internal/spacemedia"
)

type ordinaryLossFunc func(context.Context, ordinarycall.PartyEligibilityLoss) error

func (f ordinaryLossFunc) ApplyPartyEligibilityLoss(c context.Context, l ordinarycall.PartyEligibilityLoss) error {
	return f(c, l)
}

type spaceLossFunc func(context.Context, spacemedia.PartyEligibilityLoss) error

func (f spaceLossFunc) ApplyPartyEligibilityLoss(c context.Context, l spacemedia.PartyEligibilityLoss) error {
	return f(c, l)
}

func nativeLossTarget() LossTarget {
	return LossTarget{SourceKind: "ORDINARY", RoomID: uuid.New(), Incarnation: uuid.New(), Reason: "SESSION_REVOKED", Member: Member{AccountID: uuid.New(), ProfileID: uuid.New(), OperationID: uuid.New(), Generation: uuid.NewString(), SessionEpoch: 7, State: Joined}}
}

func TestNativeNormalizerPreservesExactOrdinaryLossAndCASOutcome(t *testing.T) {
	loss := nativeLossTarget()
	for _, outcome := range []error{nil, ordinarycall.ErrPartySourceChanged, errors.New("database failed")} {
		calls := 0
		n := &NativeNormalizer{Ordinary: ordinaryLossFunc(func(_ context.Context, got ordinarycall.PartyEligibilityLoss) error {
			calls++
			require.Equal(t, ordinarycall.PartyEligibilityLoss{RoomID: loss.RoomID, Incarnation: loss.Incarnation, AccountID: loss.Member.AccountID, ProfileID: loss.Member.ProfileID, OperationID: loss.Member.OperationID, Generation: uuid.MustParse(loss.Member.Generation), SessionEpoch: loss.Member.SessionEpoch}, got)
			return outcome
		})}
		err := n.ApplyEligibilityLoss(context.Background(), loss)
		require.Equal(t, 1, calls)
		if outcome == nil {
			require.NoError(t, err)
		} else if errors.Is(outcome, ordinarycall.ErrPartySourceChanged) {
			require.ErrorIs(t, err, ErrSourceChanged)
		} else {
			require.ErrorIs(t, err, ErrUnavailable)
		}
	}
}

func TestNativeNormalizerPreservesOpaqueSpaceLossAndCASOutcome(t *testing.T) {
	loss := nativeLossTarget()
	loss.SourceKind = "SPACE"
	loss.Member.Generation = "opaque/native epoch / alpha"
	for _, outcome := range []error{nil, spacemedia.ErrPartySourceChanged, errors.New("database failed")} {
		calls := 0
		n := &NativeNormalizer{Space: spaceLossFunc(func(_ context.Context, got spacemedia.PartyEligibilityLoss) error {
			calls++
			require.Equal(t, spacemedia.PartyEligibilityLoss{RoomID: loss.RoomID, Incarnation: loss.Incarnation, AccountID: loss.Member.AccountID, ProfileID: loss.Member.ProfileID, OperationID: loss.Member.OperationID, Generation: loss.Member.Generation, SessionEpoch: loss.Member.SessionEpoch}, got)
			return outcome
		})}
		err := n.ApplyEligibilityLoss(context.Background(), loss)
		require.Equal(t, 1, calls)
		if outcome == nil {
			require.NoError(t, err)
		} else if errors.Is(outcome, spacemedia.ErrPartySourceChanged) {
			require.ErrorIs(t, err, ErrSourceChanged)
		} else {
			require.ErrorIs(t, err, ErrUnavailable)
		}
	}
}

func TestNativeNormalizerRejectsUnknownOrUnsupportedEvidenceBeforeWriter(t *testing.T) {
	for _, mutate := range []func(*LossTarget){
		func(l *LossTarget) { l.SourceKind = "foreign" }, func(l *LossTarget) { l.RoomID = uuid.Nil }, func(l *LossTarget) { l.Incarnation = uuid.Nil },
		func(l *LossTarget) { l.Member.AccountID = uuid.Nil }, func(l *LossTarget) { l.Member.ProfileID = uuid.Nil }, func(l *LossTarget) { l.Member.OperationID = uuid.Nil },
		func(l *LossTarget) { l.Member.SessionEpoch = 0 }, func(l *LossTarget) { l.Member.Generation = "not a native UUID" }, func(l *LossTarget) { l.Member.Generation = "" },
		func(l *LossTarget) { l.Member.State = Reconnecting }, func(l *LossTarget) { l.Reason = "RECONNECT_EXPIRED" }, func(l *LossTarget) { l.Reason = "" },
		func(l *LossTarget) { l.SourceKind = "SPACE"; l.Member.Generation = string([]byte{255}) },
	} {
		loss := nativeLossTarget()
		mutate(&loss)
		n := &NativeNormalizer{Ordinary: ordinaryLossFunc(func(context.Context, ordinarycall.PartyEligibilityLoss) error {
			t.Fatal("unsupported evidence reached writer")
			return nil
		}), Space: spaceLossFunc(func(context.Context, spacemedia.PartyEligibilityLoss) error {
			t.Fatal("unsupported evidence reached writer")
			return nil
		})}
		require.ErrorIs(t, n.ApplyEligibilityLoss(context.Background(), loss), ErrUnavailable)
	}
	var absent *NativeNormalizer
	require.ErrorIs(t, absent.ApplyEligibilityLoss(context.Background(), nativeLossTarget()), ErrUnavailable)
	require.ErrorIs(t, (&NativeNormalizer{}).ApplyEligibilityLoss(context.Background(), nativeLossTarget()), ErrUnavailable)
}
