package grpcsvc

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	rolev1 "voice.app/voice/role/v1"
)

func TestGlobalRoleTerminalRequiresQualifiedRootAndExactOwnResult(t *testing.T) {
	root, _ := deletionRoleSource(t)
	q, err := buildGlobalRoleTerminalRequest(root)
	require.NoError(t, err)
	require.Equal(t, root.BotID.String(), q.BotId)
	require.Equal(t, root.OperationID.String(), q.DecisionId)
	good := &rolev1.RegisterBotGlobalTerminalResponse{Receipt: &rolev1.BotGlobalTerminalReceipt{ProtocolVersion: 1, Participant: "role", BotId: q.BotId, DecisionId: q.DecisionId, ReceiptId: uuid.NewString(), RegisteredAt: timestamppb.Now(), Outcome: rolev1.BotGlobalTerminalOutcome_BOT_GLOBAL_TERMINAL_OUTCOME_BOT_ACTOR_ROLE_MUTATIONS_FENCED}}
	require.NoError(t, validateGlobalRoleTerminalReceipt(q, good))
	for _, mode := range []string{"nil", "missing receipt", "other participant", "other Bot", "other root", "receipt identity", "missing time", "invalid time", "unknown response", "unknown receipt", "unknown time", "unknown outcome"} {
		t.Run(mode, func(t *testing.T) {
			r := proto.Clone(good).(*rolev1.RegisterBotGlobalTerminalResponse)
			switch mode {
			case "nil":
				r = nil
			case "missing receipt":
				r.Receipt = nil
			case "other participant":
				r.Receipt.Participant = "space"
			case "other Bot":
				r.Receipt.BotId = uuid.NewString()
			case "other root":
				r.Receipt.DecisionId = uuid.NewString()
			case "receipt identity":
				r.Receipt.ReceiptId = uuid.Nil.String()
			case "missing time":
				r.Receipt.RegisteredAt = nil
			case "invalid time":
				r.Receipt.RegisteredAt.Seconds = -1
			case "unknown response":
				r.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			case "unknown receipt":
				r.Receipt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			case "unknown time":
				r.Receipt.RegisteredAt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			case "unknown outcome":
				r.Receipt.Outcome = rolev1.BotGlobalTerminalOutcome(2)
			}
			require.Error(t, validateGlobalRoleTerminalReceipt(q, r))
		})
	}
	for _, mode := range []string{"nil", "ordinary uninstall", "wrong root", "changed decision hash"} {
		t.Run(mode, func(t *testing.T) {
			op, _ := deletionRoleSource(t)
			switch mode {
			case "nil":
				op = nil
			case "ordinary uninstall":
				op.Purpose = "space_uninstall"
			case "wrong root":
				op.OperationID = uuid.New()
			case "changed decision hash":
				op.DecisionSHA256[0] ^= 1
			}
			_, err := buildGlobalRoleTerminalRequest(op)
			require.Error(t, err)
		})
	}
}
