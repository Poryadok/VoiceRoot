package spaceevents

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

// BE038 T3: additions must retain the existing envelope and legacy empty update.
func TestOrdinaryContract_AdditivePayloads(t *testing.T) {
	update := (&eventsv1.SpaceUpdated{}).ProtoReflect().Descriptor()
	changed := update.Fields().ByName("changed_fields")
	require.NotNil(t, changed, "ordinary updates must identify actual changed fields")
	require.Equal(t, protoreflect.Repeated, changed.Cardinality())
	require.Equal(t, protoreflect.StringKind, changed.Kind())
	require.Equal(t, protoreflect.FieldNumber(1), update.Fields().ByName("space_id").Number())
	arm := (&eventsv1.ChatStreamEvent{}).ProtoReflect().Descriptor().Fields().ByName("space_member_banned")
	require.NotNil(t, arm, "effective bans require an additive internal domain fact")
	for _, name := range []protoreflect.Name{"space_id", "account_id", "banned_by_profile_id"} {
		require.NotNil(t, arm.Message().Fields().ByName(name))
	}
	require.Equal(t, protoreflect.FieldNumber(17), (&eventsv1.ChatStreamEvent{}).ProtoReflect().Descriptor().Fields().ByName("space_updated").Number())
}

func TestOrdinaryContract_StableAdditiveWireAndUnknownPreservation(t *testing.T) {
	const spaceID = "11111111-1111-1111-1111-111111111111"
	legacy := &eventsv1.SpaceUpdated{SpaceId: spaceID}
	legacyBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(legacy)
	require.NoError(t, err)
	require.Equal(t, append([]byte{0x0a, 36}, []byte(spaceID)...), legacyBytes,
		"an empty additive field must not rewrite legacy ownership update bytes")
	var old eventsv1.SpaceUpdated
	require.NoError(t, proto.Unmarshal(legacyBytes, &old))
	require.Empty(t, old.GetChangedFields(), "legacy empty update remains valid full invalidation")

	for _, envelope := range []*eventsv1.ChatStreamEvent{
		{Payload: &eventsv1.ChatStreamEvent_SpaceUpdated{SpaceUpdated: &eventsv1.SpaceUpdated{
			SpaceId: spaceID, ChangedFields: []string{"description", "name"},
		}}},
		{Payload: &eventsv1.ChatStreamEvent_SpaceMemberBanned{SpaceMemberBanned: &eventsv1.SpaceMemberBanned{
			SpaceId: spaceID, AccountId: "22222222-2222-2222-2222-222222222222", BannedByProfileId: "33333333-3333-3333-3333-333333333333",
		}}},
	} {
		envelope.EventId = "44444444-4444-4444-4444-444444444444"
		envelope.OccurredAt = timestamppb.New(time.Date(2026, 10, 18, 12, 0, 0, 123456000, time.UTC))
		envelope.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
		first, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
		require.NoError(t, err)
		var restored eventsv1.ChatStreamEvent
		require.NoError(t, proto.Unmarshal(first, &restored))
		require.True(t, proto.Equal(envelope, &restored))
		second, err := proto.MarshalOptions{Deterministic: true}.Marshal(&restored)
		require.NoError(t, err)
		require.Equal(t, first, second, "fixed event identity/time and unknown bytes survive round-trip")
	}
}

// BE038 T12: internal wire compatibility does not authorize logging invite codes.
func TestOrdinaryContract_InviteCodeNeverLogged(t *testing.T) {
	server := startJSTestServer(t)
	bootstrapStream(t, server.ClientURL())
	publisher, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	var logs bytes.Buffer
	publisher.Logger = slog.New(slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	const code = "privileged-invite-code-never-log"
	require.NoError(t, publisher.PublishInviteCreated(context.Background(), "11111111-1111-1111-1111-111111111111", code))
	require.NotEmpty(t, logs.String(), "exercise the actual success logging path")
	require.NotContains(t, logs.String(), code)
	require.NotContains(t, logs.String(), "invite_code")
}
