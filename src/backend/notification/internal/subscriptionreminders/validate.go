package subscriptionreminders

import (
	"math"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
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

func decode(raw []byte) (*eventsv1.SubscriptionStreamEvent, error) {
	e := new(eventsv1.SubscriptionStreamEvent)
	if len(raw) == 0 || len(raw) > 1<<20 || proto.Unmarshal(raw, e) != nil || e.ProtocolVersion != 1 || e.AggregateRevision == 0 || e.AggregateRevision > math.MaxInt64 || !validID(e.EventId) || !validID(e.AggregateId) || !validTime(e.OccurredAt) || unknown(e.ProtoReflect()) {
		return nil, ErrContractMismatch
	}
	if e.AggregateKind != eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL && e.AggregateKind != eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE {
		return nil, ErrContractMismatch
	}
	if r := e.GetGraceReminder(); r != nil {
		if !validID(r.AccountId) || (r.Day != 1 && r.Day != 3 && r.Day != 7) {
			return nil, ErrContractMismatch
		}
		if e.AggregateKind == eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL {
			if r.AccountId != e.AggregateId || r.Plan != "premium" || r.SpaceId != nil {
				return nil, ErrContractMismatch
			}
		} else if r.Plan != "space_pro" || r.SpaceId == nil || r.GetSpaceId() != e.AggregateId {
			return nil, ErrContractMismatch
		}
		return e, nil
	}
	s := e.GetEntitlementChanged()
	if s == nil || !validID(s.EntitlementId) || !validID(s.DeletionFenceId) || !validTime(s.EffectiveAt) || !validTime(s.CurrentPeriodEnd) || !validTime(s.EntitledUntil) {
		return nil, ErrContractMismatch
	}
	if s.Reason <= eventsv1.EntitlementReason_ENTITLEMENT_REASON_UNSPECIFIED || s.Reason > eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED {
		return nil, ErrContractMismatch
	}
	if (s.DowngradeCycleId != nil && !validID(*s.DowngradeCycleId)) || (s.DeletionCycleId != nil && !validID(*s.DeletionCycleId)) {
		return nil, ErrContractMismatch
	}
	if (s.DeletionCycleId == nil) != (s.PurgeAt == nil) || (s.PurgeAt != nil && !validTime(s.PurgeAt)) || (s.GracePeriodEnd != nil && !validTime(s.GracePeriodEnd)) {
		return nil, ErrContractMismatch
	}
	if s.Reason == eventsv1.EntitlementReason_ENTITLEMENT_REASON_ACCOUNT_DELETE_SCHEDULED && (e.AggregateKind != eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL || s.State != eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE || s.DeletionCycleId == nil || s.PurgeAt == nil || !s.PurgeAt.AsTime().Equal(s.EffectiveAt.AsTime().Add(30*24*time.Hour))) {
		return nil, ErrContractMismatch
	}
	if s.Reason == eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED && !s.PurchaserDeleted {
		return nil, ErrContractMismatch
	}
	switch e.AggregateKind {
	case eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL:
		if s.AccountId != e.AggregateId || s.SpaceId != "" || s.Plan != "premium" || s.PurchaserAccountId != nil || s.PurchaserDeleted || s.Reason == eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED {
			return nil, ErrContractMismatch
		}
	case eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE:
		if s.SpaceId != e.AggregateId || s.AccountId != "" || s.Plan != "space_pro" {
			return nil, ErrContractMismatch
		}
		if s.PurchaserDeleted {
			if s.PurchaserAccountId != nil {
				return nil, ErrContractMismatch
			}
		} else if s.PurchaserAccountId == nil || !validID(*s.PurchaserAccountId) {
			return nil, ErrContractMismatch
		}
	}
	switch s.State {
	case eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE:
		if !proto.Equal(s.EntitledUntil, s.CurrentPeriodEnd) || !s.EntitledUntil.AsTime().After(s.EffectiveAt.AsTime()) {
			return nil, ErrContractMismatch
		}
	case eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD:
		if !validTime(s.GracePeriodEnd) || !proto.Equal(s.EntitledUntil, s.GracePeriodEnd) || !s.GracePeriodEnd.AsTime().Equal(s.EffectiveAt.AsTime().Add(7*24*time.Hour)) {
			return nil, ErrContractMismatch
		}
	case eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE:
		if !proto.Equal(s.EntitledUntil, s.EffectiveAt) {
			return nil, ErrContractMismatch
		}
	default:
		return nil, ErrContractMismatch
	}
	return e, nil
}
