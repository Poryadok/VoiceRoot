package roleeventwire

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

func wireFixture(t *testing.T, subject string) *eventsv1.RoleStreamEvent {
	t.Helper()
	envelope := &eventsv1.RoleStreamEvent{EventId: "abcdefab-1111-4111-8111-111111111111", OccurredAt: timestamppb.New(time.Unix(1700000000, 0))}
	mutation := &eventsv1.RoleMutation{SpaceId: "10000000-0000-4000-8000-000000000001", RoleId: "20000000-0000-4000-8000-000000000002", ProfileId: "30000000-0000-4000-8000-000000000003", ChatId: "40000000-0000-4000-8000-000000000004", VoiceRoomId: "50000000-0000-4000-8000-000000000005", ChangedFields: []string{"name", "permissions"}}
	field := envelope.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(strings.ReplaceAll(subject, ".", "_")))
	require.NotNil(t, field)
	envelope.ProtoReflect().Set(field, protoreflect.ValueOfMessage(mutation.ProtoReflect()))
	return envelope
}

func TestDecodeRequiresEachActionDataField(t *testing.T) {
	for _, subject := range []string{"role.created", "role.updated", "role.deleted", "role.assigned", "role.revoked", "role.chat_override_set", "role.chat_override_removed", "role.voice_override_set", "role.voice_override_removed"} {
		required := []string{"space_id", "role_id"}
		switch subject {
		case "role.assigned", "role.revoked":
			required = append(required, "profile_id")
		case "role.chat_override_set", "role.chat_override_removed":
			required = append(required, "chat_id")
		case "role.voice_override_set", "role.voice_override_removed":
			required = append(required, "voice_room_id")
		}
		for _, name := range required {
			t.Run(subject+"_without_"+name, func(t *testing.T) {
				envelope := wireFixture(t, subject)
				arm := envelope.ProtoReflect().WhichOneof(envelope.ProtoReflect().Descriptor().Oneofs().ByName("payload"))
				mutation := envelope.ProtoReflect().Get(arm).Message()
				field := mutation.Descriptor().Fields().ByName(protoreflect.Name(name))
				mutation.Clear(field)
				data, err := proto.Marshal(envelope)
				require.NoError(t, err)
				_, err = Decode(subject, data)
				require.ErrorIs(t, err, ErrInvalid)
			})
		}
	}
}

func TestDecodeRetainsChangedFieldsAndLegacyUncertainty(t *testing.T) {
	envelope := wireFixture(t, "role.updated")
	binary, err := proto.Marshal(envelope)
	require.NoError(t, err)
	// A legal unknown group can precede the envelope, even with a JSON-looking
	// first byte. Classification must not reject otherwise valid protobuf.
	binary = append([]byte{0x7b, 0x7c}, binary...)
	event, err := Decode("role.updated", binary)
	require.NoError(t, err)
	require.False(t, event.Legacy)
	require.Equal(t, envelope.EventId, event.EventID)
	require.Equal(t, envelope.OccurredAt.AsTime(), event.OccurredAt.AsTime())
	require.Equal(t, []string{"name", "permissions"}, event.Mutation.ChangedFields)
	legacy, err := Decode("role.updated", []byte(`{"role_id":"legacy-role","changed_fields":["name"],"future":{"value":true}}`))
	require.NoError(t, err)
	require.True(t, legacy.Legacy)
	require.Empty(t, legacy.EventID)
	require.Nil(t, legacy.OccurredAt)
	require.Equal(t, []string{"name"}, legacy.Mutation.ChangedFields)
	missing, err := Decode("role.updated", []byte(`{"role_id":"legacy-role"}`))
	require.NoError(t, err)
	require.Nil(t, missing.Mutation.ChangedFields)
	require.Empty(t, missing.Mutation.SpaceId, "legacy missing-field tolerance is retained")
}

func TestDecodeRejectsNonCanonicalOrZeroEventID(t *testing.T) {
	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "ABCDEFAB-1111-4111-8111-111111111111", "not-uuid"} {
		envelope := wireFixture(t, "role.updated")
		envelope.EventId = id
		data, err := proto.Marshal(envelope)
		require.NoError(t, err)
		_, err = Decode("role.updated", data)
		require.ErrorIs(t, err, ErrInvalid)
	}
}
