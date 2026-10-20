package grpcsvc

import (
	"encoding/hex"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestBotNativePurgeWireIndependentGolden(t *testing.T) {
	// Codec sentinels are not accepted business identities or proof authority.
	manifest := &commonv1.ManifestBinding{ManifestId: "manifest", ManifestSha256: []byte{1, 2}, ItemCount: 2}
	native := &spacev1.RemoveBotMemberRequest{SpaceId: "space", ProfileId: "actor", InstallOperationId: "add", InstallationIntent: &spacev1.BotInstallationIntent{OperationId: "cancel", BotId: "bot", InstallationId: "install", SpaceId: "space", ActorProfileId: "actor", Generation: 3}}
	request := &spacev1.ReconcileBotPurgeInstallationRequest{ProtocolVersion: 1, OperationId: "drain", SpaceId: "space", DeletionOperationId: "delete", Generation: 8, Manifest: manifest, BotId: "bot", InstallationId: "install", NativeCancellation: native}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	require.NoError(t, err)
	require.Equal(t, "08011205647261696e1a057370616365220664656c657465280832100a086d616e69666573741202010218023a03626f744207696e7374616c6c4a3b0a05737061636512056163746f721a260a0663616e63656c1203626f741a07696e7374616c6c220573706163652a056163746f7230032203616464", hex.EncodeToString(raw))
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	require.Equal(t, "sha256:cc87bf818f5f879cc0feda83f304c913b1844dba12bacb3f43ac636d701a31b4", hash)
	digest, err := hex.DecodeString("cc87bf818f5f879cc0feda83f304c913b1844dba12bacb3f43ac636d701a31b4")
	require.NoError(t, err)
	receipt := &spacev1.BotPurgeInstallationReceipt{ProtocolVersion: 1, ReceiptId: "receipt", OperationId: "drain", RequestSha256: digest, SpaceId: "space", DeletionOperationId: "delete", Generation: 8, Manifest: manifest, BotId: "bot", InstallationId: "install", NativeCancellation: native, Disposition: spacev1.BotPurgeInstallationDisposition_BOT_PURGE_INSTALLATION_DISPOSITION_NATIVE_NEGATIVE_INTENT_FENCED, CompletedAt: &timestamppb.Timestamp{Seconds: 1}}
	raw, err = (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	require.NoError(t, err)
	require.Equal(t, "08011207726563656970741a05647261696e2220cc87bf818f5f879cc0feda83f304c913b1844dba12bacb3f43ac636d701a31b42a057370616365320664656c657465380842100a086d616e69666573741202010218024a03626f745207696e7374616c6c5a3b0a05737061636512056163746f721a260a0663616e63656c1203626f741a07696e7374616c6c220573706163652a056163746f723003220361646460036a020801", hex.EncodeToString(raw))
	decoded := new(spacev1.BotPurgeInstallationReceipt)
	require.NoError(t, proto.Unmarshal(raw, decoded))
	require.True(t, proto.Equal(receipt, decoded))
	descriptor := request.ProtoReflect().Descriptor()
	require.True(t, descriptor.ReservedRanges().Has(10))
	require.Nil(t, descriptor.Fields().ByNumber(10))
	for name, tag := range map[protoreflect.Name]protoreflect.FieldNumber{"protocol_version": 1, "operation_id": 2, "space_id": 3, "deletion_operation_id": 4, "generation": 5, "manifest": 6, "bot_id": 7, "installation_id": 8, "native_cancellation": 9} {
		require.Equal(t, tag, descriptor.Fields().ByName(name).Number())
	}
	for _, name := range []protoreflect.Name{"owner_account_id", "decision_sha256", "global_native_adds_fenced", "deleted_role_count"} {
		require.Nil(t, receipt.ProtoReflect().Descriptor().Fields().ByName(name))
	}
	for _, name := range []protoreflect.Name{"SetEntryPolicy", "AddBotMember", "RemoveBotMember", "ApplyBotInstallationRemoval", "GetBotPurgeHandoff", "RegisterBotGlobalTerminal", "ReconcileBotPurgeInstallation"} {
		require.NotNil(t, spacev1.File_voice_space_v1_space_proto.Services().ByName("SpaceService").Methods().ByName(name))
	}
}
