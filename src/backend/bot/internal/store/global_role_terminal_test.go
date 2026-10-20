package store_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
)

func TestBotGlobalRoleTerminalRejectsCrossParticipantAndRetainsExactReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip("real PostgreSQL Role terminal storage boundary")
	}
	st := startBotStore(t)
	ctx := t.Context()
	bot, _, err := st.CreateBot(ctx, uuid.New(), "Role terminal storage", "", `[]`, uuid.New())
	require.NoError(t, err)
	root, err := st.BeginBotDeletion(ctx, bot.ID, bot.OwnerAccountID)
	require.NoError(t, err)
	claim, err := st.ClaimBotDeletion(ctx, root.OperationID)
	require.NoError(t, err)
	q := &rolev1.RegisterBotGlobalTerminalRequest{ProtocolVersion: 1, BotId: bot.ID.String(), DecisionId: root.OperationID.String()}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	require.NoError(t, st.SealBotGlobalRoleRequest(ctx, claim, body))
	good := &rolev1.RegisterBotGlobalTerminalResponse{Receipt: &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: q.BotId, DecisionId: q.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}}
	for _, mode := range []string{"participant", "root", "outcome", "time", "unknown"} {
		t.Run(mode, func(t *testing.T) {
			r := proto.Clone(good).(*rolev1.RegisterBotGlobalTerminalResponse)
			switch mode {
			case "participant":
				r.Receipt.Participant = "space"
			case "root":
				r.Receipt.DecisionId = uuid.NewString()
			case "outcome":
				r.Receipt.Outcome = rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_UNSPECIFIED
			case "time":
				r.Receipt.RegisteredAt = nil
			case "unknown":
				r.Receipt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			}
			raw, e := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
			require.NoError(t, e)
			require.ErrorIs(t, st.SaveBotGlobalRoleReceipt(ctx, claim, body, raw), store.ErrInstallationConflict)
		})
	}
	other := &spacev1.RegisterBotGlobalTerminalResponse{Receipt: &spacev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, BotId: q.BotId, DecisionId: q.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: spacev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_GLOBAL_NATIVE_ADDS_FENCED}}
	spaceBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(other)
	require.NoError(t, err)
	require.ErrorIs(t, st.SaveBotGlobalRoleReceipt(ctx, claim, body, spaceBytes), store.ErrInstallationConflict)
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(good)
	require.NoError(t, err)
	require.NoError(t, st.SaveBotGlobalRoleReceipt(ctx, claim, body, raw))
	require.NoError(t, st.SaveBotGlobalRoleReceipt(ctx, claim, body, raw))
	effects, err := st.GlobalTerminalEffects(ctx, claim)
	require.NoError(t, err)
	require.Equal(t, "COMPLETE", effects[0].State)
	require.Equal(t, raw, effects[0].ReceiptBytes)
	require.Equal(t, "PENDING", effects[1].State)
	require.Nil(t, effects[1].ReceiptBytes)
	require.ErrorIs(t, st.CompleteBotDeletion(ctx, claim), store.ErrDeletionPending, "own Role result alone cannot settle Space")
}
