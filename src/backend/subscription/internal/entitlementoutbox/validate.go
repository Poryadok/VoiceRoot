// Package entitlementoutbox provides the source-disabled Subscription producer
// durability seam. Live adapters must not use it before lifecycle ordering,
// purge, snapshot bootstrap and consumer cutover are implemented.
package entitlementoutbox

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

const Subject = "subscription.entitlement_changed"

var (
	ErrContractMismatch = errors.New("subscription entitlement contract mismatch")
	ErrRevisionConflict = errors.New("subscription entitlement revision conflict")
	ErrLeaseLost        = errors.New("subscription outbox lease lost")
)

func validID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}

func validTime(t *timestamppb.Timestamp) bool { return t != nil && t.CheckValid() == nil }

func unknown(m protoreflect.Message) bool {
	if len(m.GetUnknown()) != 0 {
		return true
	}
	found := false
	m.Range(func(f protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if f.Kind() == protoreflect.MessageKind && unknown(v.Message()) {
			found = true
			return false
		}
		return true
	})
	return found
}

// Validate rejects ambiguous or unsupported authority before any durable write.
// Provider ordering and legal state transitions are owned by the future adapter
// state machine; this validates a complete snapshot, not provider arrival order.
func Validate(e *eventsv1.SubscriptionStreamEvent) error {
	if e == nil || e.ProtocolVersion != 1 || e.AggregateRevision == 0 || e.AggregateRevision > math.MaxInt64 || !validID(e.EventId) || !validID(e.AggregateId) || !validTime(e.OccurredAt) || unknown(e.ProtoReflect()) || proto.Size(e) > 1<<20 {
		return ErrContractMismatch
	}
	s := e.GetEntitlementChanged()
	if s == nil || !validID(s.EntitlementId) || !validID(s.DeletionFenceId) || !validTime(s.EffectiveAt) || !validTime(s.CurrentPeriodEnd) || !validTime(s.EntitledUntil) {
		return ErrContractMismatch
	}
	if s.Reason <= eventsv1.EntitlementReason_ENTITLEMENT_REASON_UNSPECIFIED || s.Reason > eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED {
		return ErrContractMismatch
	}
	if (s.DowngradeCycleId != nil && !validID(*s.DowngradeCycleId)) || (s.DeletionCycleId != nil && !validID(*s.DeletionCycleId)) {
		return ErrContractMismatch
	}
	if (s.DeletionCycleId == nil) != (s.PurgeAt == nil) || (s.PurgeAt != nil && !validTime(s.PurgeAt)) || (s.GracePeriodEnd != nil && !validTime(s.GracePeriodEnd)) {
		return ErrContractMismatch
	}
	if s.Reason == eventsv1.EntitlementReason_ENTITLEMENT_REASON_ACCOUNT_DELETE_SCHEDULED &&
		(e.AggregateKind != eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL ||
			s.State != eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE || s.DeletionCycleId == nil || s.PurgeAt == nil ||
			!s.PurgeAt.AsTime().Equal(s.EffectiveAt.AsTime().Add(30*24*time.Hour))) {
		return ErrContractMismatch
	}
	if s.Reason == eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED && !s.PurchaserDeleted {
		return ErrContractMismatch
	}
	switch e.AggregateKind {
	case eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL:
		if s.AccountId != e.AggregateId || s.SpaceId != "" || s.Plan != "premium" || s.PurchaserAccountId != nil || s.PurchaserDeleted || s.Reason == eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED {
			return ErrContractMismatch
		}
	case eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE:
		if s.SpaceId != e.AggregateId || s.AccountId != "" || s.Plan != "space_pro" {
			return ErrContractMismatch
		}
		if s.PurchaserDeleted {
			if s.PurchaserAccountId != nil {
				return ErrContractMismatch
			}
		} else if s.PurchaserAccountId == nil || !validID(*s.PurchaserAccountId) {
			return ErrContractMismatch
		}
	default:
		return ErrContractMismatch
	}
	switch s.State {
	case eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE:
		if !proto.Equal(s.EntitledUntil, s.CurrentPeriodEnd) || !s.EntitledUntil.AsTime().After(s.EffectiveAt.AsTime()) {
			return ErrContractMismatch
		}
	case eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD:
		if !validTime(s.GracePeriodEnd) || !proto.Equal(s.EntitledUntil, s.GracePeriodEnd) || !s.EntitledUntil.AsTime().After(s.EffectiveAt.AsTime()) {
			return ErrContractMismatch
		}
	case eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE:
		if !proto.Equal(s.EntitledUntil, s.EffectiveAt) {
			return ErrContractMismatch
		}
	default:
		return ErrContractMismatch
	}
	return nil
}
