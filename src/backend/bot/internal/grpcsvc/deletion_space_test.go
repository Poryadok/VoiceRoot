package grpcsvc

import (
	"crypto/sha256"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
)

func spaceDeletionSource(t *testing.T) (*store.BotDeletion, store.BotDeletionEffect) {
	t.Helper()
	op, e := deletionRoleSource(t)
	e.Kind = "SPACE_CLEANUP"
	return op, e
}

func TestSpaceDeletionRequestPreservesRootAndOriginalCancellation(t *testing.T) {
	op, e := spaceDeletionSource(t)
	q, err := buildSpaceDeletionRequest(op, e)
	require.NoError(t, err)
	require.Equal(t, e.EffectID.String(), q.OperationId)
	require.Equal(t, op.OperationID.String(), q.DecisionId)
	require.Equal(t, e.Target.CancelOperationID.String(), q.NativeCancellation.InstallationIntent.OperationId)
	require.NotEqual(t, q.OperationId, q.NativeCancellation.InstallationIntent.OperationId)
	require.Equal(t, e.Target.InstallOperationID.String(), q.NativeCancellation.InstallOperationId)
	require.Equal(t, e.Target.InstallationID.String(), q.NativeCancellation.InstallationIntent.InstallationId)
	require.Equal(t, uint64(e.Target.BotGeneration), q.NativeCancellation.InstallationIntent.Generation)
	for _, mode := range []string{"legacy", "missing cancellation", "changed scope", "wrong effect", "chat target"} {
		t.Run(mode, func(t *testing.T) {
			d, target := spaceDeletionSource(t)
			switch mode {
			case "legacy":
				target.Target.LegacyEvidence = true
				sealMessagingTarget(t, &target)
			case "missing cancellation":
				target.Target.CancelOperationID = uuid.Nil
				sealMessagingTarget(t, &target)
			case "changed scope":
				target.Target.ScopeSHA256[0] ^= 1
			case "wrong effect":
				target.Kind = "ROLE_REMOVE"
			case "chat target":
				chat := uuid.New()
				target.ChatID = &chat
			}
			_, err := buildSpaceDeletionRequest(d, target)
			require.Error(t, err)
		})
	}
}

func spaceDeletionReceipt(t *testing.T, q *spacev1.ApplyBotInstallationRemovalRequest) *spacev1.ApplyBotInstallationRemovalResponse {
	t.Helper()
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	h := sha256.Sum256(body)
	nested, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q.NativeCancellation)
	require.NoError(t, err)
	nh := sha256.Sum256(nested)
	return &spacev1.ApplyBotInstallationRemovalResponse{Receipt: &spacev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, EvidenceKind: "native", EvidenceSha256: nh[:], FrozenScopeSha256: q.FrozenScopeSha256, RequestSha256: h[:], Participant: "space", Outcome: "COMPLETED", CompletedAt: timestamppb.New(time.Now()), GlobalNativeAddsFenced: true}}
}

func TestSpaceDeletionReceiptRequiresExactTypedGlobalFence(t *testing.T) {
	op, e := spaceDeletionSource(t)
	q, err := buildSpaceDeletionRequest(op, e)
	require.NoError(t, err)
	good := spaceDeletionReceipt(t, q)
	require.NoError(t, validateSpaceDeletionReceipt(q, good))
	for _, mode := range []string{"Empty", "no terminal fence", "wrong root", "wrong request", "wrong cancellation", "wrong actor", "wrong Space", "wrong purpose", "wrong participant", "unknown receipt", "missing completion time"} {
		t.Run(mode, func(t *testing.T) {
			r := proto.Clone(good).(*spacev1.ApplyBotInstallationRemovalResponse)
			switch mode {
			case "Empty":
				r.Receipt = nil
			case "no terminal fence":
				r.Receipt.GlobalNativeAddsFenced = false
			case "wrong root":
				r.Receipt.DecisionId = uuid.NewString()
			case "wrong request":
				r.Receipt.RequestSha256[0] ^= 1
			case "wrong cancellation":
				r.Receipt.EvidenceSha256[0] ^= 1
			case "wrong actor":
				r.Receipt.ActorProfileId = uuid.NewString()
			case "wrong Space":
				r.Receipt.SpaceId = uuid.NewString()
			case "wrong purpose":
				r.Receipt.Purpose = "space_uninstall"
			case "wrong participant":
				r.Receipt.Participant = "bot"
			case "unknown receipt":
				r.Receipt.ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			case "missing completion time":
				r.Receipt.CompletedAt = nil
			}
			require.Error(t, validateSpaceDeletionReceipt(q, r))
		})
	}
	// Recovery must revalidate the exact typed evidence before skipping this row.
	e.State = "COMPLETE"
	e.RequestBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	e.ReceiptBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(good)
	require.NoError(t, err)
	require.NoError(t, validateCompletedDeletionEffect(op, e))
	good.Receipt.GlobalNativeAddsFenced = false
	e.ReceiptBytes, err = (proto.MarshalOptions{Deterministic: true}).Marshal(good)
	require.NoError(t, err)
	require.Error(t, validateCompletedDeletionEffect(op, e))
}
