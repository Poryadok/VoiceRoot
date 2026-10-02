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
	require.Equal(t, 5, service.Methods().Len(), "private GIS listener exposes provisioning, close, roster apply, conversion fencing and activation")
	method := service.Methods().ByName("ProvisionGameSessionRoom")
	require.NotNil(t, method)
	require.Equal(t, "ProvisionGameSessionRoomRequest", string(method.Input().Name()))
	require.Equal(t, "ProvisionGameSessionRoomResponse", string(method.Output().Name()))
	require.Equal(t, "/voice.calls.v1.GameSessionProvisioningService/ProvisionGameSessionRoom", "/"+string(service.FullName())+"/"+string(method.Name()))
	closeMethod := service.Methods().ByName("CloseGameSessionRoom")
	require.NotNil(t, closeMethod)
	require.Equal(t, "CloseGameSessionRoomRequest", string(closeMethod.Input().Name()))
	require.Equal(t, "CloseGameSessionRoomResponse", string(closeMethod.Output().Name()))
	applyRoster := service.Methods().ByName("ApplyGameSessionRoster")
	require.NotNil(t, applyRoster)
	require.Equal(t, "ApplyGameSessionRosterRequest", string(applyRoster.Input().Name()))
	require.Equal(t, "ApplyGameSessionRosterResponse", string(applyRoster.Output().Name()))
	for _, name := range []string{"operation_id", "application_id", "environment_id", "session_id", "voice_room_id", "roster_revision", "profile_ids", "lease_expires_at"} {
		require.NotNil(t, applyRoster.Input().Fields().ByName(protoreflect.Name(name)), name)
	}
	for _, name := range []string{"receipt_id", "request_hash", "accepted_revision", "lease_expires_at"} {
		require.NotNil(t, applyRoster.Output().Fields().ByName(protoreflect.Name(name)), name)
	}
}

func TestGameSessionProvisioningRequest_BindsOperationScopeResourceAndChatReceipt(t *testing.T) {
	message := callsv1.File_voice_calls_v1_calls_proto.Messages().ByName("ProvisionGameSessionRoomRequest")
	require.NotNil(t, message)
	for _, field := range []string{"operation_id", "application_id", "environment_id", "session_id", "resource", "chat_id", "chat_creation_operation_id"} {
		require.NotNil(t, message.Fields().ByName(protoreflect.Name(field)), field)
	}
	response := callsv1.File_voice_calls_v1_calls_proto.Messages().ByName("ProvisionGameSessionRoomResponse")
	require.NotNil(t, response.Fields().ByName("session_id"), "owner receipt must bind the GIS session identity")
	resource := callsv1.File_voice_calls_v1_calls_proto.Messages().ByName("GameSessionResourceRef")
	require.NotNil(t, resource)
	require.NotNil(t, resource.Fields().ByName("kind"))
	require.NotNil(t, resource.Fields().ByName("external_resource_key"))
}
