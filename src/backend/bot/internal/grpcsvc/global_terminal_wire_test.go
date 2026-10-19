package grpcsvc

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestBotGlobalTerminalWireIndependentGolden(t *testing.T) {
	// Codec sentinels only: accepted business identities must be canonical UUIDs.
	q := &spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: "bot", DecisionId: "terminal"}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	require.Equal(t, "08011203626f741a087465726d696e616c", hex.EncodeToString(body))
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	require.Equal(t, "sha256:e9f41cf0f2dcdb8f9b995a45b576ffa03adcee93883393d54b514d4fc8342664", hash)
	fields := q.ProtoReflect().Descriptor().Fields()
	require.Equal(t, 3, fields.Len(), "minimal root assertion carries no personal/source digest")
	for name, number := range map[protoreflect.Name]protoreflect.FieldNumber{"protocol_version": 1, "bot_id": 2, "decision_id": 3} {
		require.Equal(t, number, fields.ByName(name).Number())
	}
	receipt := &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: "bot", DecisionId: "terminal", ReceiptId: "receipt", RegisteredAt: &timestamppb.Timestamp{Seconds: 1}, Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(receipt)
	require.NoError(t, err)
	require.Equal(t, "08011203626f741a087465726d696e616c2207726563656970742a0208013001", hex.EncodeToString(raw))
	rf := receipt.ProtoReflect().Descriptor().Fields()
	require.Equal(t, 6, rf.Len())
	require.Equal(t, protoreflect.FullName("google.protobuf.Timestamp"), rf.ByName("registered_at").Message().FullName())
	require.Equal(t, protoreflect.EnumNumber(0), spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED.Number())
	require.Equal(t, protoreflect.EnumNumber(1), receipt.Outcome.Number())
	method := spacev1.File_voice_space_v1_space_proto.Services().ByName("SpaceService").Methods().ByName("RegisterBotGlobalTerminal")
	require.Equal(t, q.ProtoReflect().Descriptor().FullName(), method.Input().FullName())
	require.Equal(t, protoreflect.FullName("voice.space.v1.RegisterBotGlobalTerminalResponse"), method.Output().FullName())
	for _, name := range []protoreflect.Name{"SetEntryPolicy", "AddBotMember", "RemoveBotMember", "ApplyBotInstallationRemoval", "GetBotPurgeHandoff"} {
		require.NotNil(t, spacev1.File_voice_space_v1_space_proto.Services().ByName("SpaceService").Methods().ByName(name))
	}
}
