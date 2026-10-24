package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	spacev1 "voice.app/voice/space/v1"
)

func TestCommunicationStateWireHasExplicitVersionAndBoundSnapshot(t *testing.T) {
	t.Parallel()
	service := spacev1.File_voice_space_v1_space_proto.Services().ByName("SpaceService")
	require.NotNil(t, service)
	method := service.Methods().ByName("ResolveMemberCommunicationState")
	require.NotNil(t, method, "Messaging needs a protected timeout decision, not a default false bool")
	for _, name := range []protoreflect.Name{"protocol_version", "space_id", "profile_id"} {
		require.NotNil(t, method.Input().Fields().ByName(name), "request binding %s", name)
	}
	voice := service.Methods().ByName("ResolveVoiceRoomAccess")
	for _, message := range []protoreflect.MessageDescriptor{method.Output(), voice.Output()} {
		for _, name := range []protoreflect.Name{"protocol_version", "space_id", "profile_id", "access_epoch", "evaluated_at", "communication_state", "timed_out_until"} {
			require.NotNil(t, message.Fields().ByName(name), "snapshot evidence %s.%s", message.Name(), name)
		}
		state := message.Fields().ByName("communication_state")
		require.Equal(t, protoreflect.EnumKind, state.Kind())
		for name, value := range map[protoreflect.Name]protoreflect.EnumNumber{
			"MEMBER_COMMUNICATION_STATE_UNSPECIFIED": 0,
			"MEMBER_COMMUNICATION_STATE_CLEAR":       1,
			"MEMBER_COMMUNICATION_STATE_TIMED_OUT":   2,
		} {
			enum := state.Enum().Values().ByName(name)
			require.NotNil(t, enum)
			require.Equal(t, value, enum.Number())
		}
	}
}
