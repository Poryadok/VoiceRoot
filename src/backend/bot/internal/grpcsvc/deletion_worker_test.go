package grpcsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/store"
)

func TestDeletionWorkerRevalidatesHistoricalTypedCompletion(t *testing.T) {
	op, effect := messagingDeletionSource(t)
	q, err := buildMessagingDeletionRequest(op, effect)
	require.NoError(t, err)
	effect.State = "COMPLETE"
	effect.RequestBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	response := messagingReceipt(t, q)
	effect.ReceiptBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	require.NoError(t, err)
	require.NoError(t, validateCompletedDeletionEffect(op, effect))
	for _, mode := range []string{"changed request", "corrupt target", "Space Empty"} {
		t.Run(mode, func(t *testing.T) {
			candidate := effect
			switch mode {
			case "changed request":
				candidate.RequestBytes = []byte("changed request")
			case "corrupt target":
				bad := proto.Clone(response).(*messagingv1.ApplyBotChatPinRemovalResponse)
				bad.Receipt.Targets[0].CleanupRevision++
				candidate.ReceiptBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(bad)
				require.NoError(t, err)
			case "Space Empty":
				candidate.Kind = "SPACE_CLEANUP"
				candidate.ReceiptBytes = nil
			}
			err := validateCompletedDeletionEffect(op, candidate)
			if mode == "Space Empty" {
				require.ErrorIs(t, err, store.ErrDeletionPending)
			} else {
				require.Error(t, err)
			}
		})
	}
}
