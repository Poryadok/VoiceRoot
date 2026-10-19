package grpcsvc

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestBotPurgeHandoffWireIndependentGolden(t *testing.T) {
	// Short codec sentinels are not accepted business identities or receipts.
	manifest := &commonv1.ManifestBinding{ManifestId: "manifest", ManifestSha256: []byte{1, 2}, ItemCount: 2}
	space := &spacev1.GetBotPurgeHandoffRequest{ProtocolVersion: 1, RequestId: "read", SpaceId: "space", DeletionOperationId: "delete", Generation: 8, Manifest: manifest}
	messaging := &messagingv1.GetBotPurgeChatCoverageRequest{ProtocolVersion: 1, RequestId: "read", SpaceId: "space", DeletionOperationId: "delete", Generation: 8, Manifest: manifest, ChatIds: []string{"a", "b"}}
	for _, sample := range []struct {
		q          proto.Message
		body, hash string
	}{
		{space, "08011204726561641a057370616365220664656c657465280832100a086d616e6966657374120201021802", "sha256:fad7f5bb501af11d7383d6fab613e91030202b7d208eb5566e57b208e53b7732"},
		{messaging, "08011204726561641a057370616365220664656c657465280832100a086d616e69666573741202010218023a01613a0162", "sha256:6792765c8e900d03e0b086ee71c3e6942186fa05674bab35c555d42233974170"},
	} {
		body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(sample.q)
		require.NoError(t, err)
		require.Equal(t, sample.body, hex.EncodeToString(body))
		hash, err := principal.RequestHash(sample.q)
		require.NoError(t, err)
		require.Equal(t, sample.hash, hash)
	}
	for _, entry := range []proto.Message{&spacev1.BotPurgeRoleHandoff{}, &spacev1.BotPurgeMessagingHandoff{}} {
		fields := entry.ProtoReflect().Descriptor().Fields()
		for name, number := range map[protoreflect.Name]protoreflect.FieldNumber{"original_receipt": 1, "original_request_bytes": 2, "original_request_sha256": 3, "original_receipt_bytes": 4, "original_receipt_sha256": 5} {
			require.Equal(t, number, fields.ByName(name).Number())
		}
		require.Nil(t, fields.ByName("complete"), "absence is never synthesized participant completion")
	}
	role := (&spacev1.BotPurgeRoleHandoff{}).ProtoReflect().Descriptor().Fields().ByName("original_receipt")
	require.Equal(t, protoreflect.FullName("voice.role.v1.RetireSpaceReceipt"), role.Message().FullName())
	msg := (&spacev1.BotPurgeMessagingHandoff{}).ProtoReflect().Descriptor().Fields().ByName("original_receipt")
	require.Equal(t, protoreflect.FullName("voice.common.v1.SpacePurgeReceipt"), msg.Message().FullName())
	response := (&messagingv1.GetBotPurgeChatCoverageResponse{}).ProtoReflect().Descriptor().Fields()
	require.Nil(t, response.ByName("receipt_id"), "membership observation is not a cleanup receipt")
	require.Equal(t, protoreflect.FieldNumber(8), response.ByName("coverage").Number())
	require.Equal(t, protoreflect.FieldNumber(9), response.ByName("request_sha256").Number())
	for _, name := range []protoreflect.Name{"SetEntryPolicy", "AddBotMember", "RemoveBotMember", "ApplyBotInstallationRemoval", "GetBotPurgeHandoff"} {
		require.NotNil(t, spacev1.File_voice_space_v1_space_proto.Services().ByName("SpaceService").Methods().ByName(name))
	}
	require.Equal(t, protoreflect.FieldNumber(14), (&messagingv1.ApplyBotChatPinRemovalRequest{}).ProtoReflect().Descriptor().Fields().ByName("attachments").Number())
}
