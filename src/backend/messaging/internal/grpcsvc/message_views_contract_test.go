package grpcsvc

import (
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"testing"
	messagingv1 "voice.app/voice/messaging/v1"
)

func TestMessageViewsWireContract(t *testing.T) {
	file := messagingv1.File_voice_messaging_v1_messaging_proto
	rpc := file.Services().ByName("MessagingService").Methods().ByName("RecordMessageView")
	require.NotNil(t, rpc, "viewport views require a dedicated idempotent RPC")
	field := file.Messages().ByName("Message").Fields().ByName("view_count")
	require.NotNil(t, field, "history must restore durable counters after reconnect")
	require.Equal(t, protoreflect.Uint64Kind, field.Kind())
	require.True(t, field.HasPresence(), "DM must omit the group/channel counter")
	require.Equal(t, protoreflect.FullName("voice.messaging.v1.RecordMessageViewResponse"), rpc.Output().FullName())
}
