package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	messagingv1 "voice.app/voice/messaging/v1"
)

func TestSchedulingProtoContractKeepsAdditiveFieldNumbers(t *testing.T) {
	t.Helper()

	sendRequest := (&messagingv1.SendMessageRequest{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FieldNumber(12), sendRequest.Fields().ByName("scheduled_at").Number())
	require.Equal(t, protoreflect.FieldNumber(13), sendRequest.Fields().ByName("send_when_online").Number())
	require.Equal(t, "delivery_schedule", string(sendRequest.Fields().ByName("scheduled_at").ContainingOneof().Name()))

	sendResponse := (&messagingv1.SendMessageResponse{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FieldNumber(2), sendResponse.Fields().ByName("scheduled_message").Number())

	update := (&messagingv1.UpdateScheduledMessageRequest{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FieldNumber(2), update.Fields().ByName("payload").Number())
	require.Equal(t, protoreflect.FieldNumber(3), update.Fields().ByName("scheduled_at").Number())
	require.Equal(t, protoreflect.FieldNumber(4), update.Fields().ByName("send_when_online").Number())

	service := messagingv1.File_voice_messaging_v1_messaging_proto.Services().ByName("MessagingService")
	for _, name := range []protoreflect.Name{
		"ListScheduledMessages",
		"UpdateScheduledMessage",
		"CancelScheduledMessage",
		"SendScheduledMessageNow",
	} {
		require.NotNil(t, service.Methods().ByName(name), "missing scheduling RPC %s", name)
	}
}
