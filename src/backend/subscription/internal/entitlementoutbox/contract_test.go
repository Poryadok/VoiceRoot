package entitlementoutbox_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

// Reflection keeps this RED contract compilable before additive stubs exist.
// The accepted source is subscription-lifecycle-convergence-exec-plan.md.
func TestSubscriptionSnapshotContract_EnvelopeAndCompleteSnapshot(t *testing.T) {
	envelope := (&eventsv1.SubscriptionStreamEvent{}).ProtoReflect().Descriptor()
	for name, kind := range map[protoreflect.Name]protoreflect.Kind{
		"protocol_version":   protoreflect.Uint32Kind,
		"aggregate_kind":     protoreflect.EnumKind,
		"aggregate_id":       protoreflect.StringKind,
		"aggregate_revision": protoreflect.Uint64Kind,
	} {
		t.Run(string(name), func(t *testing.T) {
			contractField(t, envelope, name, kind)
		})
	}
	arm := contractField(t, envelope, "entitlement_changed", protoreflect.MessageKind)
	require.NotNil(t, arm.ContainingOneof(), "snapshot must be an additive payload arm")
	require.Equal(t, protoreflect.Name("payload"), arm.ContainingOneof().Name())
	require.Greater(t, int(arm.Number()), 18, "existing legacy payload numbers are reserved")
	snapshot := arm.Message()
	for _, name := range []protoreflect.Name{
		"entitlement_id", "plan", "account_id", "space_id", "purchaser_account_id",
		"deletion_fence_id", "deletion_cycle_id", "downgrade_cycle_id",
	} {
		contractField(t, snapshot, name, protoreflect.StringKind)
	}
	for _, name := range []protoreflect.Name{"purchaser_deleted", "cancel_at_period_end"} {
		contractField(t, snapshot, name, protoreflect.BoolKind)
	}
	for _, name := range []protoreflect.Name{
		"effective_at", "entitled_until", "current_period_end", "grace_period_end", "purge_at",
	} {
		field := contractField(t, snapshot, name, protoreflect.MessageKind)
		require.Equal(t, protoreflect.FullName("google.protobuf.Timestamp"), field.Message().FullName())
	}
	for _, name := range []protoreflect.Name{"purchaser_account_id", "deletion_cycle_id", "downgrade_cycle_id"} {
		require.True(t, snapshot.Fields().ByName(name).HasPresence(), "%s must preserve absence", name)
	}
	contractEnum(t, contractField(t, envelope, "aggregate_kind", protoreflect.EnumKind).Enum(), "PERSONAL", "SPACE")
	contractEnum(t, contractField(t, snapshot, "state", protoreflect.EnumKind).Enum(), "ACTIVE", "GRACE_PERIOD", "INACTIVE")
	contractEnum(t, contractField(t, snapshot, "reason", protoreflect.EnumKind).Enum(),
		"STARTED", "RENEWED", "PAYMENT_FAILED", "PAYMENT_RECOVERED", "CANCEL_SCHEDULED",
		"CANCEL_RESUMED", "PERIOD_ENDED", "GRACE_EXPIRED", "ACCOUNT_DELETE_SCHEDULED",
		"ACCOUNT_RESTORED", "PURCHASER_PURGED")
}

func TestSubscriptionSnapshotContract_RoundTrip(t *testing.T) {
	for _, scenario := range []struct {
		name, aggregate, state, reason string
		deletedPurchaser, deletion     bool
	}{
		{name: "personal active cancellation", aggregate: "PERSONAL", state: "ACTIVE", reason: "CANCEL_SCHEDULED"},
		{name: "personal grace", aggregate: "PERSONAL", state: "GRACE_PERIOD", reason: "PAYMENT_FAILED"},
		{name: "personal deletion", aggregate: "PERSONAL", state: "INACTIVE", reason: "ACCOUNT_DELETE_SCHEDULED", deletion: true},
		{name: "space grace", aggregate: "SPACE", state: "GRACE_PERIOD", reason: "PAYMENT_FAILED"},
		{name: "space retains paid state after purchaser purge", aggregate: "SPACE", state: "ACTIVE", reason: "PURCHASER_PURGED", deletedPurchaser: true},
		{name: "space expires without purchaser identity", aggregate: "SPACE", state: "INACTIVE", reason: "PERIOD_ENDED", deletedPurchaser: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			envelope := (&eventsv1.SubscriptionStreamEvent{}).ProtoReflect()
			arm := contractField(t, envelope.Descriptor(), "entitlement_changed", protoreflect.MessageKind)
			snapshot := envelope.NewField(arm).Message()
			setContractString(t, envelope, "event_id", "10000000-0000-4000-8000-000000000001")
			setContractString(t, envelope, "aggregate_id", "20000000-0000-4000-8000-000000000001")
			envelope.Set(contractField(t, envelope.Descriptor(), "protocol_version", protoreflect.Uint32Kind), protoreflect.ValueOfUint32(1))
			envelope.Set(contractField(t, envelope.Descriptor(), "aggregate_revision", protoreflect.Uint64Kind), protoreflect.ValueOfUint64(42))
			setContractEnum(t, envelope, "aggregate_kind", scenario.aggregate)
			setContractString(t, snapshot, "entitlement_id", "30000000-0000-4000-8000-000000000001")
			setContractString(t, snapshot, "deletion_fence_id", "40000000-0000-4000-8000-000000000001")
			setContractEnum(t, snapshot, "state", scenario.state)
			setContractEnum(t, snapshot, "reason", scenario.reason)
			if scenario.aggregate == "PERSONAL" {
				setContractString(t, snapshot, "plan", "premium")
				setContractString(t, snapshot, "account_id", "20000000-0000-4000-8000-000000000001")
			} else {
				setContractString(t, snapshot, "plan", "space_pro")
				setContractString(t, snapshot, "space_id", "20000000-0000-4000-8000-000000000001")
				if !scenario.deletedPurchaser {
					setContractString(t, snapshot, "purchaser_account_id", "50000000-0000-4000-8000-000000000001")
				}
			}
			snapshot.Set(contractField(t, snapshot.Descriptor(), "purchaser_deleted", protoreflect.BoolKind), protoreflect.ValueOfBool(scenario.deletedPurchaser))
			base := time.Date(2026, 9, 15, 4, 0, 0, 123456000, time.UTC)
			periodEnd, entitledUntil := base.Add(24*time.Hour), base.Add(24*time.Hour)
			if scenario.state == "GRACE_PERIOD" {
				periodEnd, entitledUntil = base, base.Add(7*24*time.Hour)
				setContractTime(t, snapshot, "grace_period_end", entitledUntil)
			}
			if scenario.state == "INACTIVE" {
				entitledUntil = base
			}
			if scenario.reason == "CANCEL_SCHEDULED" || scenario.state == "GRACE_PERIOD" {
				setContractString(t, snapshot, "downgrade_cycle_id", "60000000-0000-4000-8000-000000000001")
			}
			snapshot.Set(contractField(t, snapshot.Descriptor(), "cancel_at_period_end", protoreflect.BoolKind), protoreflect.ValueOfBool(scenario.reason == "CANCEL_SCHEDULED" || scenario.deletedPurchaser))
			if scenario.deletion {
				setContractString(t, snapshot, "deletion_cycle_id", "70000000-0000-4000-8000-000000000001")
				setContractTime(t, snapshot, "purge_at", base.Add(30*24*time.Hour))
			}
			setContractTime(t, envelope, "occurred_at", base)
			setContractTime(t, snapshot, "effective_at", base)
			setContractTime(t, snapshot, "current_period_end", periodEnd)
			setContractTime(t, snapshot, "entitled_until", entitledUntil)
			envelope.Set(arm, protoreflect.ValueOfMessage(snapshot))
			wire, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope.Interface())
			require.NoError(t, err)
			decoded := &eventsv1.SubscriptionStreamEvent{}
			require.NoError(t, proto.Unmarshal(wire, decoded))
			require.True(t, proto.Equal(envelope.Interface(), decoded), "all complete snapshot values must survive the generated decoder")
			actual := decoded.ProtoReflect().Get(arm).Message()
			if scenario.deletedPurchaser {
				require.False(t, actual.Has(actual.Descriptor().Fields().ByName("purchaser_account_id")), "purged payer must stay absent while Space remains addressable")
				require.Equal(t, "20000000-0000-4000-8000-000000000001", actual.Get(actual.Descriptor().Fields().ByName("space_id")).String())
			}
		})
	}
}

func TestSubscriptionSnapshotContract_LegacyPlanStartedWireCompatibility(t *testing.T) {
	// Build bytes from the original field numbers, independently of generated code.
	started := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "20000000-0000-4000-8000-000000000001")
	started = protowire.AppendString(protowire.AppendTag(started, 2, protowire.BytesType), "premium")
	wire := protowire.AppendString(protowire.AppendTag(nil, 1, protowire.BytesType), "10000000-0000-4000-8000-000000000001")
	wire = protowire.AppendBytes(protowire.AppendTag(wire, 10, protowire.BytesType), started)
	envelope := &eventsv1.SubscriptionStreamEvent{}
	require.NoError(t, proto.Unmarshal(wire, envelope))
	require.Equal(t, "10000000-0000-4000-8000-000000000001", envelope.GetEventId())
	require.NotNil(t, envelope.GetPlanStarted())
	require.Equal(t, "20000000-0000-4000-8000-000000000001", envelope.GetPlanStarted().GetAccountId())
	require.Equal(t, "premium", envelope.GetPlanStarted().GetPlan())
	encoded, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	require.NoError(t, err)
	require.Equal(t, wire, encoded, "the additive snapshot cannot rewrite legacy payloads")
}

func contractField(t *testing.T, descriptor protoreflect.MessageDescriptor, name protoreflect.Name, kind protoreflect.Kind) protoreflect.FieldDescriptor {
	t.Helper()
	field := descriptor.Fields().ByName(name)
	require.NotNil(t, field, "%s lacks canonical field %s", descriptor.FullName(), name)
	require.Equal(t, kind, field.Kind(), "%s.%s", descriptor.FullName(), name)
	require.False(t, field.IsList(), "%s must be singular", name)
	return field
}

func contractEnumNumber(t *testing.T, enum protoreflect.EnumDescriptor, suffix string) protoreflect.EnumNumber {
	t.Helper()
	for i := 0; i < enum.Values().Len(); i++ {
		value := enum.Values().Get(i)
		if strings.HasSuffix(string(value.Name()), "_"+suffix) || string(value.Name()) == suffix {
			return value.Number()
		}
	}
	t.Fatalf("%s lacks canonical value %s", enum.FullName(), suffix)
	return 0
}

func contractEnum(t *testing.T, enum protoreflect.EnumDescriptor, values ...string) {
	t.Helper()
	require.Equal(t, len(values)+1, enum.Values().Len(), "%s must contain only the closed canonical values and UNSPECIFIED", enum.FullName())
	require.Equal(t, protoreflect.EnumNumber(0), contractEnumNumber(t, enum, "UNSPECIFIED"))
	seen := map[protoreflect.EnumNumber]bool{0: true}
	for _, value := range values {
		number := contractEnumNumber(t, enum, value)
		require.False(t, seen[number], "canonical state/reason %s must have its own nonzero value", value)
		seen[number] = true
	}
}

func setContractString(t *testing.T, message protoreflect.Message, name protoreflect.Name, value string) {
	t.Helper()
	message.Set(contractField(t, message.Descriptor(), name, protoreflect.StringKind), protoreflect.ValueOfString(value))
}

func setContractEnum(t *testing.T, message protoreflect.Message, name protoreflect.Name, value string) {
	t.Helper()
	field := contractField(t, message.Descriptor(), name, protoreflect.EnumKind)
	message.Set(field, protoreflect.ValueOfEnum(contractEnumNumber(t, field.Enum(), value)))
}

func setContractTime(t *testing.T, message protoreflect.Message, name protoreflect.Name, value time.Time) {
	t.Helper()
	field := contractField(t, message.Descriptor(), name, protoreflect.MessageKind)
	require.Equal(t, protoreflect.FullName("google.protobuf.Timestamp"), field.Message().FullName())
	message.Set(field, protoreflect.ValueOfMessage(timestamppb.New(value).ProtoReflect()))
}
