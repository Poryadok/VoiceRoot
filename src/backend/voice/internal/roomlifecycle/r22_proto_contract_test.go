package roomlifecycle_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	callsv1 "voice.app/voice/calls/v1"
	eventsv1 "voice.app/voice/events/v1"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
)

type r22FieldSpec struct {
	number   protoreflect.FieldNumber
	kind     protoreflect.Kind
	typeName protoreflect.FullName
	optional bool
}

func requireMessage(t *testing.T, file protoreflect.FileDescriptor, name protoreflect.Name) protoreflect.MessageDescriptor {
	t.Helper()
	message := file.Messages().ByName(name)
	require.NotNil(t, message, "%s must define %s", file.Path(), name)
	return message
}

func requireMethod(t *testing.T, service protoreflect.ServiceDescriptor, name protoreflect.Name) protoreflect.MethodDescriptor {
	t.Helper()
	method := service.Methods().ByName(name)
	require.NotNil(t, method, "%s must define %s", service.FullName(), name)
	return method
}

func requireExactFields(t *testing.T, message protoreflect.MessageDescriptor, expected map[protoreflect.Name]r22FieldSpec) {
	t.Helper()
	require.Equal(t, len(expected), message.Fields().Len(), "%s field allowlist is frozen", message.FullName())
	for name, spec := range expected {
		field := message.Fields().ByName(name)
		require.NotNil(t, field, "%s must define %s", message.FullName(), name)
		require.Equal(t, spec.number, field.Number(), "%s.%s field number", message.FullName(), name)
		require.Equal(t, spec.kind, field.Kind(), "%s.%s wire kind", message.FullName(), name)
		require.Equal(t, protoreflect.Optional, field.Cardinality(), "%s.%s cardinality", message.FullName(), name)
		require.Equal(t, spec.optional, field.HasOptionalKeyword(), "%s.%s explicit optional", message.FullName(), name)
		switch spec.kind {
		case protoreflect.MessageKind:
			require.NotNil(t, field.Message())
			require.Equal(t, spec.typeName, field.Message().FullName(), "%s.%s message target", message.FullName(), name)
		case protoreflect.EnumKind:
			require.NotNil(t, field.Enum())
			require.Equal(t, spec.typeName, field.Enum().FullName(), "%s.%s enum target", message.FullName(), name)
		}
	}
}

func requireEnum(t *testing.T, file protoreflect.FileDescriptor, name protoreflect.Name, expected map[protoreflect.Name]protoreflect.EnumNumber) {
	t.Helper()
	enum := file.Enums().ByName(name)
	require.NotNil(t, enum, "%s must define %s", file.Path(), name)
	require.Equal(t, len(expected), enum.Values().Len(), "%s value allowlist is frozen", enum.FullName())
	for valueName, number := range expected {
		value := enum.Values().ByName(valueName)
		require.NotNil(t, value, "%s must define %s", enum.FullName(), valueName)
		require.Equal(t, number, value.Number())
	}
}

func TestR22ProtoContract_CallsLifecycleIsIdempotentAndTyped(t *testing.T) {
	file := callsv1.File_voice_calls_v1_calls_proto
	service := file.Services().ByName("VoiceService")
	require.NotNil(t, service)

	stringField := func(number protoreflect.FieldNumber) r22FieldSpec {
		return r22FieldSpec{number: number, kind: protoreflect.StringKind}
	}
	spaceField := func(number protoreflect.FieldNumber) r22FieldSpec {
		return r22FieldSpec{number: number, kind: protoreflect.MessageKind, typeName: "voice.space.v1.SpaceRef"}
	}
	requireExactFields(t, requireMessage(t, file, "JoinVoiceRoomRequest"), map[protoreflect.Name]r22FieldSpec{
		"voice_room_id": stringField(1), "space": spaceField(2), "operation_id": stringField(3),
	})
	requireExactFields(t, requireMessage(t, file, "LeaveVoiceRoomRequest"), map[protoreflect.Name]r22FieldSpec{
		"voice_room_id": stringField(1), "space": spaceField(2), "operation_id": stringField(3),
	})
	requireExactFields(t, requireMessage(t, file, "MoveToVoiceRoomRequest"), map[protoreflect.Name]r22FieldSpec{
		"from_voice_room_id": stringField(1), "to_voice_room_id": stringField(2), "space": spaceField(3), "operation_id": stringField(4),
	})

	moderator := requireMethod(t, service, "MoveVoiceRoomParticipant")
	require.Equal(t, protoreflect.FullName("voice.calls.v1.MoveVoiceRoomParticipantRequest"), moderator.Input().FullName())
	require.Equal(t, protoreflect.FullName("voice.calls.v1.MoveVoiceRoomParticipantResponse"), moderator.Output().FullName())
	requireExactFields(t, moderator.Input(), map[protoreflect.Name]r22FieldSpec{
		"from_voice_room_id": stringField(1), "to_voice_room_id": stringField(2), "space": spaceField(3),
		"participant_profile_id": stringField(4), "operation_id": stringField(5),
	})
}

func TestR22ProtoContract_LifecycleReceiptHasClosedTerminalBinding(t *testing.T) {
	file := callsv1.File_voice_calls_v1_calls_proto
	requireEnum(t, file, "VoiceRoomLifecycleMethod", map[protoreflect.Name]protoreflect.EnumNumber{
		"VOICE_ROOM_LIFECYCLE_METHOD_UNSPECIFIED":    0,
		"VOICE_ROOM_LIFECYCLE_METHOD_JOIN":           1,
		"VOICE_ROOM_LIFECYCLE_METHOD_LEAVE":          2,
		"VOICE_ROOM_LIFECYCLE_METHOD_SELF_MOVE":      3,
		"VOICE_ROOM_LIFECYCLE_METHOD_MODERATOR_MOVE": 4,
	})
	requireEnum(t, file, "VoiceRoomLifecycleOutcome", map[protoreflect.Name]protoreflect.EnumNumber{
		"VOICE_ROOM_LIFECYCLE_OUTCOME_UNSPECIFIED": 0,
		"VOICE_ROOM_LIFECYCLE_OUTCOME_JOINED":      1,
		"VOICE_ROOM_LIFECYCLE_OUTCOME_LEFT":        2,
		"VOICE_ROOM_LIFECYCLE_OUTCOME_MOVED":       3,
		"VOICE_ROOM_LIFECYCLE_OUTCOME_NO_OP":       4,
	})
	receipt := requireMessage(t, file, "VoiceRoomLifecycleReceipt")
	requireExactFields(t, receipt, map[protoreflect.Name]r22FieldSpec{
		"operation_id":               {number: 1, kind: protoreflect.StringKind},
		"actor_profile_id":           {number: 2, kind: protoreflect.StringKind},
		"subject_profile_id":         {number: 3, kind: protoreflect.StringKind},
		"space":                      {number: 4, kind: protoreflect.MessageKind, typeName: "voice.space.v1.SpaceRef"},
		"method":                     {number: 5, kind: protoreflect.EnumKind, typeName: "voice.calls.v1.VoiceRoomLifecycleMethod"},
		"outcome":                    {number: 6, kind: protoreflect.EnumKind, typeName: "voice.calls.v1.VoiceRoomLifecycleOutcome"},
		"source_voice_room_id":       {number: 7, kind: protoreflect.StringKind, optional: true},
		"destination_voice_room_id":  {number: 8, kind: protoreflect.StringKind, optional: true},
		"room_id":                    {number: 9, kind: protoreflect.StringKind, optional: true},
		"source_roster_version":      {number: 10, kind: protoreflect.Uint64Kind, optional: true},
		"destination_roster_version": {number: 11, kind: protoreflect.Uint64Kind, optional: true},
		"media_epoch":                {number: 12, kind: protoreflect.StringKind, optional: true},
		"space_access_epoch":         {number: 13, kind: protoreflect.Uint64Kind, optional: true},
		"role_policy_epoch":          {number: 14, kind: protoreflect.Uint64Kind, optional: true},
		"authorization_digest":       {number: 15, kind: protoreflect.BytesKind, optional: true},
	})
	for i := 0; i < receipt.Fields().Len(); i++ {
		name := strings.ToLower(string(receipt.Fields().Get(i).Name()))
		for _, forbidden := range []string{"jwt", "bearer", "token", "credential", "grant", "permission"} {
			require.NotContains(t, name, forbidden, "completed receipts are identity-bound history, never reusable authority")
		}
	}

	requireExactFields(t, requireMessage(t, file, "JoinVoiceRoomResponse"), map[protoreflect.Name]r22FieldSpec{
		"voice_session": {number: 1, kind: protoreflect.MessageKind, typeName: "voice.calls.v1.VoiceSession"},
		"receipt":       {number: 2, kind: protoreflect.MessageKind, typeName: "voice.calls.v1.VoiceRoomLifecycleReceipt"},
	})
	requireExactFields(t, requireMessage(t, file, "LeaveVoiceRoomResponse"), map[protoreflect.Name]r22FieldSpec{
		"receipt": {number: 1, kind: protoreflect.MessageKind, typeName: "voice.calls.v1.VoiceRoomLifecycleReceipt"},
	})
	requireExactFields(t, requireMessage(t, file, "MoveToVoiceRoomResponse"), map[protoreflect.Name]r22FieldSpec{
		"voice_session": {number: 1, kind: protoreflect.MessageKind, typeName: "voice.calls.v1.VoiceSession"},
		"receipt":       {number: 2, kind: protoreflect.MessageKind, typeName: "voice.calls.v1.VoiceRoomLifecycleReceipt"},
	})
	requireExactFields(t, requireMessage(t, file, "MoveVoiceRoomParticipantResponse"), map[protoreflect.Name]r22FieldSpec{
		"receipt": {number: 1, kind: protoreflect.MessageKind, typeName: "voice.calls.v1.VoiceRoomLifecycleReceipt"},
	})

	requireExactFields(t, requireMessage(t, file, "GetJoinTokenResponse"), map[protoreflect.Name]r22FieldSpec{
		"jwt":                  {number: 1, kind: protoreflect.StringKind},
		"expires_at":           {number: 2, kind: protoreflect.MessageKind, typeName: "google.protobuf.Timestamp"},
		"livekit_url":          {number: 3, kind: protoreflect.StringKind},
		"media_epoch":          {number: 4, kind: protoreflect.StringKind},
		"space_access_epoch":   {number: 5, kind: protoreflect.Uint64Kind},
		"role_policy_epoch":    {number: 6, kind: protoreflect.Uint64Kind},
		"authorization_digest": {number: 7, kind: protoreflect.BytesKind},
	})
}

func TestR22ProtoContract_AuthorityDecisionsExposeOwnedEpochs(t *testing.T) {
	spaceFile := spacev1.File_voice_space_v1_space_proto
	requireExactFields(t, requireMessage(t, spaceFile, "ResolveVoiceRoomAccessRequest"), map[protoreflect.Name]r22FieldSpec{
		"voice_room_id": {number: 1, kind: protoreflect.StringKind},
		"profile_id":    {number: 2, kind: protoreflect.StringKind},
		"space":         {number: 3, kind: protoreflect.MessageKind, typeName: "voice.space.v1.SpaceRef"},
	})
	requireExactFields(t, requireMessage(t, spaceFile, "ResolveVoiceRoomAccessResponse"), map[protoreflect.Name]r22FieldSpec{
		"space_id":     {number: 1, kind: protoreflect.StringKind},
		"member":       {number: 2, kind: protoreflect.BoolKind},
		"active":       {number: 3, kind: protoreflect.BoolKind},
		"discoverable": {number: 4, kind: protoreflect.BoolKind},
		"access_epoch": {number: 5, kind: protoreflect.Uint64Kind},
	})

	roleFile := rolev1.File_voice_role_v1_role_proto
	method := requireMethod(t, roleFile.Services().ByName("RoleService"), "ResolveVoiceRoomGrants")
	require.Equal(t, protoreflect.FullName("voice.role.v1.ResolveVoiceRoomGrantsRequest"), method.Input().FullName())
	require.Equal(t, protoreflect.FullName("voice.role.v1.ResolveVoiceRoomGrantsResponse"), method.Output().FullName())
	requireExactFields(t, method.Input(), map[protoreflect.Name]r22FieldSpec{
		"space_id": {number: 1, kind: protoreflect.StringKind}, "voice_room_id": {number: 2, kind: protoreflect.StringKind}, "profile_id": {number: 3, kind: protoreflect.StringKind},
	})
	requireExactFields(t, method.Output(), map[protoreflect.Name]r22FieldSpec{
		"grants": {number: 1, kind: protoreflect.MessageKind, typeName: "voice.role.v1.VoiceRoomGrants"}, "policy_epoch": {number: 2, kind: protoreflect.Uint64Kind},
	})
	requireExactFields(t, requireMessage(t, roleFile, "VoiceRoomGrants"), map[protoreflect.Name]r22FieldSpec{
		"can_join": {number: 1, kind: protoreflect.BoolKind}, "can_publish_audio": {number: 2, kind: protoreflect.BoolKind},
		"can_publish_video": {number: 3, kind: protoreflect.BoolKind}, "can_publish_screen_share": {number: 4, kind: protoreflect.BoolKind},
		"can_subscribe": {number: 5, kind: protoreflect.BoolKind}, "can_mute_others": {number: 6, kind: protoreflect.BoolKind},
		"can_deafen_others": {number: 7, kind: protoreflect.BoolKind}, "can_move_others": {number: 8, kind: protoreflect.BoolKind},
		"can_use_ptt": {number: 9, kind: protoreflect.BoolKind}, "priority_speaker": {number: 10, kind: protoreflect.BoolKind},
	})
}

func TestR22ProtoContract_InvalidationsHaveStableEnvelopeAndWildcardScope(t *testing.T) {
	file := eventsv1.File_voice_events_v1_jetstream_events_proto
	chatEnvelope := requireMessage(t, file, "ChatStreamEvent")
	spaceInvalidation := requireMessage(t, file, "VoiceRoomAccessInvalidated")
	requireExactFields(t, spaceInvalidation, map[protoreflect.Name]r22FieldSpec{
		"space_id": {number: 1, kind: protoreflect.StringKind}, "voice_room_id": {number: 2, kind: protoreflect.StringKind, optional: true},
		"profile_id": {number: 3, kind: protoreflect.StringKind, optional: true}, "access_epoch": {number: 4, kind: protoreflect.Uint64Kind},
	})
	spaceArm := chatEnvelope.Fields().ByName("voice_room_access_invalidated")
	require.NotNil(t, spaceArm)
	require.Equal(t, protoreflect.FieldNumber(20), spaceArm.Number())
	require.Equal(t, protoreflect.FullName("voice.events.v1.VoiceRoomAccessInvalidated"), spaceArm.Message().FullName())
	require.Equal(t, chatEnvelope.Oneofs().ByName("payload"), spaceArm.ContainingOneof())

	roleEnvelope := requireMessage(t, file, "RoleStreamEvent")
	roleInvalidation := requireMessage(t, file, "VoiceRoomPolicyInvalidated")
	requireExactFields(t, roleInvalidation, map[protoreflect.Name]r22FieldSpec{
		"space_id": {number: 1, kind: protoreflect.StringKind}, "voice_room_id": {number: 2, kind: protoreflect.StringKind, optional: true},
		"profile_id": {number: 3, kind: protoreflect.StringKind, optional: true}, "policy_epoch": {number: 4, kind: protoreflect.Uint64Kind},
	})
	roleArm := roleEnvelope.Fields().ByName("voice_room_policy_invalidated")
	require.NotNil(t, roleArm)
	require.Equal(t, protoreflect.FieldNumber(12), roleArm.Number())
	require.Equal(t, protoreflect.FullName("voice.events.v1.VoiceRoomPolicyInvalidated"), roleArm.Message().FullName())
	require.Equal(t, roleEnvelope.Oneofs().ByName("payload"), roleArm.ContainingOneof())
}
