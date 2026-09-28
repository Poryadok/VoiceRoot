package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"

	callsv1 "voice.app/voice/calls/v1"
)

func TestGameSessionProvisioningContract_IsDedicatedSingleMethodService(t *testing.T) {
	file := callsv1.File_voice_calls_v1_calls_proto
	service := file.Services().ByName("GameSessionProvisioningService")
	require.NotNil(t, service, "GIS provisioning must have a dedicated private service")
	require.Equal(t, 1, service.Methods().Len(), "private GIS listener must expose only the provisioning method")
	method := service.Methods().ByName("ProvisionGameSessionRoom")
	require.NotNil(t, method)
	require.Equal(t, "ProvisionGameSessionRoomRequest", string(method.Input().Name()))
	require.Equal(t, "ProvisionGameSessionRoomResponse", string(method.Output().Name()))
	require.Equal(t, "/voice.calls.v1.GameSessionProvisioningService/ProvisionGameSessionRoom", "/"+string(service.FullName())+"/"+string(method.Name()))
}

func TestGameSessionProvisioningRequest_BindsOperationScopeResourceAndChatReceipt(t *testing.T) {
	message := callsv1.File_voice_calls_v1_calls_proto.Messages().ByName("ProvisionGameSessionRoomRequest")
	require.NotNil(t, message)
	for _, field := range []string{"operation_id", "application_id", "environment_id", "resource", "chat_id", "chat_creation_operation_id"} {
		require.NotNil(t, message.Fields().ByName(protoreflect.Name(field)), field)
	}
	resource := callsv1.File_voice_calls_v1_calls_proto.Messages().ByName("GameSessionResourceRef")
	require.NotNil(t, resource)
	require.NotNil(t, resource.Fields().ByName("kind"))
	require.NotNil(t, resource.Fields().ByName("external_resource_key"))
}
