package grpcsvc

import (
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestRoleGlobalTerminalWireIndependentGolden(t *testing.T) {
	// Codec sentinels only; accepted business identities remain canonical UUIDs.
	q := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: "bot", DecisionId: "terminal"}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	require.Equal(t, "08011203626f741a087465726d696e616c", hex.EncodeToString(body))
	hash, err := principal.RequestHash(q)
	require.NoError(t, err)
	require.Equal(t, "sha256:e9f41cf0f2dcdb8f9b995a45b576ffa03adcee93883393d54b514d4fc8342664", hash)
	require.NotEqual(t, rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName, spacev1.SpaceService_RegisterBotGlobalTerminal_FullMethodName, "equal minimal body bytes never grant a different signed RPC")
	fields := q.ProtoReflect().Descriptor().Fields()
	require.Equal(t, 3, fields.Len())
	for name, number := range map[protoreflect.Name]protoreflect.FieldNumber{"protocol_version": 1, "bot_id": 2, "decision_id": 3} {
		require.Equal(t, number, fields.ByName(name).Number())
	}
	r := &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: "bot", DecisionId: "terminal", ReceiptId: "receipt", RegisteredAt: &timestamppb.Timestamp{Seconds: 1}, Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	require.Equal(t, "08011204726f6c651a03626f7422087465726d696e616c2a0772656365697074320208013801", hex.EncodeToString(raw))
	require.Equal(t, 7, r.ProtoReflect().Descriptor().Fields().Len())
	for name, number := range map[protoreflect.Name]protoreflect.FieldNumber{"protocol_version": 1, "participant": 2, "bot_id": 3, "decision_id": 4, "receipt_id": 5, "registered_at": 6, "outcome": 7} {
		require.Equal(t, number, r.ProtoReflect().Descriptor().Fields().ByName(name).Number())
	}
	require.Equal(t, protoreflect.EnumNumber(0), rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED.Number())
	require.Equal(t, protoreflect.EnumNumber(1), r.Outcome.Number())
	service := rolev1.File_voice_role_v1_role_proto.Services().ByName("RoleService")
	for _, name := range []protoreflect.Name{"ApplyBotInstallationRemoval", "EnsureDefaultMemberRole", "RemoveMemberRoles", "RetireSpace", "RegisterBotGlobalTerminal"} {
		require.NotNil(t, service.Methods().ByName(name))
	}
	require.Equal(t, q.ProtoReflect().Descriptor().FullName(), service.Methods().ByName("RegisterBotGlobalTerminal").Input().FullName())
}

func TestRoleGlobalTerminalReceiptCannotSettleSpaceTerminal(t *testing.T) {
	bot, decision := uuid.NewString(), uuid.NewString()
	r := &rolev1.RegisterBotGlobalTerminalResponse{Receipt: &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: bot, DecisionId: decision, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	other := new(spacev1.RegisterBotGlobalTerminalResponse)
	if proto.Unmarshal(body, other) == nil {
		require.Error(t, validateGlobalSpaceTerminalReceipt(&spacev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: bot, DecisionId: decision}, other))
	}
}
