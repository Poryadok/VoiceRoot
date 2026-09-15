// Package providerlifecycle implements an internal, source-disabled provider
// transition engine. No live adapter may call it until protected reconciliation,
// reminder scheduling, privacy fences and consumer cutover are complete.
package providerlifecycle

import (
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

var (
	ErrContractMismatch    = errors.New("subscription provider contract mismatch")
	ErrNeedsReconciliation = errors.New("subscription provider facts need reconciliation")
)

// Command contains already verified provider facts, not public request input.
// Version must come from provider authority; arrival time is never a version.
// RawBody is retained only in the bounded durable conflict quarantine, while
// successful replay binds its hash and every normalized field.
type Command struct {
	Provider, EventID, SubscriptionID   string
	Version                             int64
	Kind                                eventsv1.SubscriptionAggregateKind
	AggregateID, PurchaserID            string
	Reason                              eventsv1.EntitlementReason
	EffectiveAt, PeriodStart, PeriodEnd time.Time
	BillingPeriod                       string
	RawBody                             []byte
}

type IDs struct{ Entitlement, DeletionFence, DowngradeCycle string }

const (
	active    = eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE
	grace     = eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD
	inactive  = eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE
	started   = eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED
	renewed   = eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED
	failed    = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED
	recovered = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_RECOVERED
	cancelled = eventsv1.EntitlementReason_ENTITLEMENT_REASON_CANCEL_SCHEDULED
	resumed   = eventsv1.EntitlementReason_ENTITLEMENT_REASON_CANCEL_RESUMED
	ended     = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PERIOD_ENDED
	expired   = eventsv1.EntitlementReason_ENTITLEMENT_REASON_GRACE_EXPIRED
)

func canonicalID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}

func validText(s string) bool {
	return len(s) > 0 && len(s) <= 512 && utf8.ValidString(s) && !strings.ContainsRune(s, 0)
}

func validateCommand(c Command) error {
	if (c.Provider != "fake" && c.Provider != "paddle" && c.Provider != "cloudpayments") || !validText(c.EventID) || !validText(c.SubscriptionID) || c.Version <= 0 || !canonicalID(c.AggregateID) || len(c.RawBody) == 0 || len(c.RawBody) > 1<<20 {
		return ErrContractMismatch
	}
	if c.BillingPeriod != "monthly" && c.BillingPeriod != "yearly" {
		return ErrContractMismatch
	}
	for _, v := range []time.Time{c.EffectiveAt, c.PeriodStart, c.PeriodEnd} {
		if v.IsZero() || timestamppb.New(v).CheckValid() != nil {
			return ErrContractMismatch
		}
	}
	if !c.PeriodEnd.After(c.PeriodStart) || c.Reason < started || c.Reason > expired {
		return ErrContractMismatch
	}
	switch c.Kind {
	case eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL:
		if c.PurchaserID != "" {
			return ErrContractMismatch
		}
	case eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE:
		if !canonicalID(c.PurchaserID) {
			return ErrContractMismatch
		}
	default:
		return ErrContractMismatch
	}
	return nil
}

// transition is deterministic and never mutates the supplied projection. Store
// allocates candidate IDs; only the winning transaction persists them.
func transition(current *eventsv1.EntitlementChanged, c Command, ids IDs) (*eventsv1.EntitlementChanged, error) {
	if err := validateCommand(c); err != nil {
		return nil, err
	}
	var s *eventsv1.EntitlementChanged
	if current == nil {
		if c.Reason != started {
			return nil, ErrNeedsReconciliation
		}
		s = &eventsv1.EntitlementChanged{EntitlementId: ids.Entitlement, DeletionFenceId: ids.DeletionFence}
		if c.Kind == 1 {
			s.AccountId = c.AggregateID
			s.Plan = "premium"
		} else {
			s.SpaceId = c.AggregateID
			s.Plan = "space_pro"
			payer := c.PurchaserID
			s.PurchaserAccountId = &payer
		}
	} else {
		if (c.Kind == 1 && (current.AccountId != c.AggregateID || current.SpaceId != "")) || (c.Kind == 2 && (current.SpaceId != c.AggregateID || current.AccountId != "" || current.GetPurchaserAccountId() != c.PurchaserID)) {
			return nil, ErrContractMismatch
		}
		if current.DeletionCycleId != nil || current.PurgeAt != nil || current.PurchaserDeleted || current.EffectiveAt == nil || current.CurrentPeriodEnd == nil || c.EffectiveAt.Before(current.EffectiveAt.AsTime()) {
			return nil, ErrNeedsReconciliation
		}
		s = proto.Clone(current).(*eventsv1.EntitlementChanged)
	}
	switch c.Reason {
	case started, renewed, recovered, resumed:
		if !c.PeriodEnd.After(c.EffectiveAt) {
			return nil, ErrNeedsReconciliation
		}
		if current != nil {
			if c.Reason == started && current.State != inactive {
				return nil, ErrNeedsReconciliation
			}
			if (c.Reason == renewed && current.State != active && current.State != grace) || (c.Reason == recovered && current.State != grace) {
				return nil, ErrNeedsReconciliation
			}
			if c.PeriodEnd.Before(current.CurrentPeriodEnd.AsTime()) {
				return nil, ErrNeedsReconciliation
			}
			if c.Reason == resumed && (current.State != active || !current.CancelAtPeriodEnd || !c.PeriodEnd.Equal(current.CurrentPeriodEnd.AsTime())) {
				return nil, ErrNeedsReconciliation
			}
		}
		s.State = active
		s.CancelAtPeriodEnd = false
		s.DowngradeCycleId = nil
		s.GracePeriodEnd = nil
		s.CurrentPeriodEnd = timestamppb.New(c.PeriodEnd)
		s.EntitledUntil = timestamppb.New(c.PeriodEnd)
	case cancelled:
		if current == nil || current.State != active || !c.PeriodEnd.Equal(current.CurrentPeriodEnd.AsTime()) || !c.PeriodEnd.After(c.EffectiveAt) {
			return nil, ErrNeedsReconciliation
		}
		if current.CancelAtPeriodEnd {
			return s, nil
		}
		s.CancelAtPeriodEnd = true
		if s.DowngradeCycleId == nil {
			cycle := ids.DowngradeCycle
			s.DowngradeCycleId = &cycle
		}
	case failed:
		if current == nil || (current.State != active && current.State != grace) || !c.PeriodEnd.Equal(current.CurrentPeriodEnd.AsTime()) {
			return nil, ErrNeedsReconciliation
		}
		if current.State == grace {
			return s, nil
		}
		s.State = grace
		s.GracePeriodEnd = timestamppb.New(c.EffectiveAt.Add(7 * 24 * time.Hour))
		s.EntitledUntil = s.GracePeriodEnd
		if s.DowngradeCycleId == nil {
			cycle := ids.DowngradeCycle
			s.DowngradeCycleId = &cycle
		}
	case ended, expired:
		if current == nil || !c.PeriodEnd.Equal(current.CurrentPeriodEnd.AsTime()) {
			return nil, ErrNeedsReconciliation
		}
		boundary := current.CurrentPeriodEnd
		if c.Reason == expired {
			if current.State != grace || current.GracePeriodEnd == nil {
				return nil, ErrNeedsReconciliation
			}
			boundary = current.GracePeriodEnd
		} else if current.State != active {
			return nil, ErrNeedsReconciliation
		}
		if !c.EffectiveAt.Equal(boundary.AsTime()) {
			return nil, ErrNeedsReconciliation
		}
		s.State = inactive
		s.EntitledUntil = timestamppb.New(c.EffectiveAt)
		if s.DowngradeCycleId == nil {
			cycle := ids.DowngradeCycle
			s.DowngradeCycleId = &cycle
		}
	default:
		return nil, ErrNeedsReconciliation
	}
	s.Reason = c.Reason
	s.EffectiveAt = timestamppb.New(c.EffectiveAt)
	return s, nil
}
