package grpcsvc

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestSpaceGlobalCleanupWirePreservesExistingRPCsAndExactTags(t *testing.T) {
	service := spacev1.File_voice_space_v1_space_proto.Services().ByName("SpaceService")
	for _, name := range []protoreflect.Name{"SetEntryPolicy", "AddBotMember", "RemoveBotMember", "GetMemberAdmissionAuthority", "ApplyBotInstallationRemoval"} {
		require.NotNil(t, service.Methods().ByName(name), "preserve full69/402e admission and Settings union")
	}
	request := (&spacev1.ApplyBotInstallationRemovalRequest{}).ProtoReflect().Descriptor()
	for name, number := range map[protoreflect.Name]protoreflect.FieldNumber{"protocol_version": 1, "operation_id": 2, "decision_id": 3, "decision_sha256": 4, "purpose": 5, "bot_id": 6, "actor_profile_id": 7, "space_id": 8, "native_cancellation": 9, "frozen_scope_sha256": 13} {
		require.Equal(t, number, request.Fields().ByName(name).Number())
	}
	require.Equal(t, 1, request.ReservedRanges().Len())
	require.Equal(t, [2]protoreflect.FieldNumber{10, 13}, request.ReservedRanges().Get(0))
	receipt := (&spacev1.BotInstallationRemovalReceipt{}).ProtoReflect().Descriptor()
	require.Equal(t, protoreflect.FieldNumber(16), receipt.Fields().ByName("global_native_adds_fenced").Number())
	require.Nil(t, receipt.Fields().ByName("removed_membership_count"), "unsupported removal counts are not asserted")
}

func TestSpaceGlobalCleanupDeterministicWireGolden(t *testing.T) {
	// Short non-UUID sentinels are a codec vector, never accepted business proof.
	q := &spacev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: "child", DecisionId: "root", DecisionSha256: []byte{1, 2}, Purpose: "global_delete", BotId: "bot", ActorProfileId: "actor", SpaceId: "space", FrozenScopeSha256: []byte{3, 4}, NativeCancellation: &spacev1.RemoveBotMemberRequest{SpaceId: "space", ProfileId: "actor", InstallOperationId: "add", InstallationIntent: &spacev1.BotInstallationIntent{OperationId: "cancel", BotId: "bot", InstallationId: "install", SpaceId: "space", ActorProfileId: "actor", Generation: 7}}}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	// Independently assembled from protobuf tags/lengths, without generated DTOs.
	require.Equal(t, "080112056368696c641a04726f6f74220201022a0d676c6f62616c5f64656c6574653203626f743a056163746f72420573706163654a3b0a05737061636512056163746f721a260a0663616e63656c1203626f741a07696e7374616c6c220573706163652a056163746f72300722036164646a020304", hex.EncodeToString(body))
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	require.Equal(t, "sha256:afedb66c893780a3f50c14324f30c54681ae0ca70a683a393073c157b54f60f4", hash)
	nested, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q.NativeCancellation)
	require.NoError(t, err)
	require.Equal(t, "0a05737061636512056163746f721a260a0663616e63656c1203626f741a07696e7374616c6c220573706163652a056163746f7230072203616464", hex.EncodeToString(nested))
	terminal, err := (proto.MarshalOptions{Deterministic: true}).Marshal(&spacev1.BotInstallationRemovalReceipt{GlobalNativeAddsFenced: true})
	require.NoError(t, err)
	require.Equal(t, "800101", hex.EncodeToString(terminal))
	require.NotEqual(t, spacev1.SpaceService_RemoveBotMember_FullMethodName, spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName)
}
