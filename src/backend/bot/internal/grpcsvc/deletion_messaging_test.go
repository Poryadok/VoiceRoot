package grpcsvc

import (
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/store"
)

func messagingDeletionSource(t *testing.T) (*store.BotDeletion, store.BotDeletionEffect) {
	t.Helper()
	op, e := deletionRoleSource(t)
	e.Kind = "MESSAGING_UNPIN"
	chat := e.Target.Chats[0]
	e.ChatID = &chat
	q := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: e.Target.SpaceID.String(), OperationId: uuid.NewString()}
	qb, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	qh := sha256.Sum256(qb)
	r := &chatv1.GetBotChatScopeAuthorityResponse{RequestSha256: qh[:], Scope: &chatv1.GetSpaceChatScopeAuthorityResponse{ProtocolVersion: 1, ChatId: q.ChatId, ExpectedSpaceId: q.ExpectedSpaceId, ActualSpaceId: q.ExpectedSpaceId, OperationId: q.OperationId, ChatScopeIncarnation: uuid.NewString(), ChatAuthorityRevision: 1, Exists: true, Live: true, ChatType: chatv1.ChatType_CHAT_TYPE_GROUP, ManagedKind: chatv1.ChatScopeManagedKind_CHAT_SCOPE_MANAGED_KIND_ORDINARY, LifecycleState: chatv1.ChatScopeLifecycleState_CHAT_SCOPE_LIFECYCLE_STATE_LIVE}}
	rb, err := (proto.MarshalOptions{Deterministic: true}).Marshal(r)
	require.NoError(t, err)
	rh := sha256.Sum256(rb)
	observation := store.ChatScopeCleanupObservation{OperationID: uuid.MustParse(q.OperationId), Request: qb, Response: rb, RequestSHA: qh[:], ResponseSHA: rh[:]}
	for range 2 {
		e.Target.ChatSources = append(e.Target.ChatSources, store.ChatScopeCleanupSource{AttachmentID: uuid.New(), ChatID: chat, InstallationID: e.Target.InstallationID, OriginalUnqualified: true, Current: &observation})
	}
	sealMessagingTarget(t, &e)
	return op, e
}
func sealMessagingTarget(t *testing.T, e *store.BotDeletionEffect) {
	t.Helper()
	var err error
	e.Target.ScopeBytes, err = json.Marshal(e.Target)
	require.NoError(t, err)
	h := sha256.Sum256(e.Target.ScopeBytes)
	e.Target.ScopeSHA256 = h[:]
}

func TestMessagingDeletionRequestPreservesFrozenAttachmentBatch(t *testing.T) {
	op, e := messagingDeletionSource(t)
	q, err := buildMessagingDeletionRequest(op, e)
	require.NoError(t, err)
	require.Len(t, q.Attachments, 2)
	require.Less(t, q.Attachments[0].AttachmentId, q.Attachments[1].AttachmentId)
	require.Equal(t, e.Target.InstallationID.String(), q.GetNative().InstallationId)
	require.True(t, q.Attachments[0].GetOriginalScopeUnqualified())
	require.NotNil(t, q.Attachments[0].CurrentScope)
	// Mutating returned protobufs cannot rewrite the frozen recovery source.
	before := append([]byte(nil), e.Target.ScopeBytes...)
	q.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation = uuid.NewString()
	again, err := buildMessagingDeletionRequest(op, e)
	require.NoError(t, err)
	require.NotEqual(t, q.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation, again.Attachments[0].CurrentScope.Response.Scope.ChatScopeIncarnation)
	require.Equal(t, before, e.Target.ScopeBytes)
	for _, mode := range []string{"missing sources", "foreign installation", "duplicate lifetime", "corrupt observation", "foreign chat", "scope hash"} {
		t.Run(mode, func(t *testing.T) {
			d, target := messagingDeletionSource(t)
			switch mode {
			case "missing sources":
				target.Target.ChatSources = nil
			case "foreign installation":
				target.Target.ChatSources[0].InstallationID = uuid.New()
			case "duplicate lifetime":
				target.Target.ChatSources[1].AttachmentID = target.Target.ChatSources[0].AttachmentID
			case "corrupt observation":
				target.Target.ChatSources[0].Current.ResponseSHA[0] ^= 1
			case "foreign chat":
				target.ChatID = new(uuid.UUID)
				*target.ChatID = uuid.New()
			case "scope hash":
				target.Target.ScopeSHA256[0] ^= 1
			}
			if mode != "scope hash" {
				sealMessagingTarget(t, &target)
			}
			_, err := buildMessagingDeletionRequest(d, target)
			require.Error(t, err)
		})
	}
}

func messagingReceipt(t *testing.T, q *messagingv1.ApplyBotChatPinRemovalRequest) *messagingv1.ApplyBotChatPinRemovalResponse {
	t.Helper()
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	require.NoError(t, err)
	h := sha256.Sum256(body)
	eb, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q.GetNative())
	require.NoError(t, err)
	eh := sha256.Sum256(eb)
	r := &messagingv1.BotChatPinRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, ChatId: q.ChatId, EvidenceKind: "native", EvidenceSha256: eh[:], FrozenScopeSha256: q.FrozenScopeSha256, RequestSha256: h[:], Participant: "messaging", Outcome: "COMPLETED", CompletedAt: timestamppb.New(time.Now()), ExecutionCleanupRevision: 3, RemovedPinCount: 1}
	for _, a := range q.Attachments {
		digest, err := messagingAttachmentDigest(q, a)
		require.NoError(t, err)
		r.Targets = append(r.Targets, &messagingv1.BotChatPinRemovalTargetReceipt{AttachmentId: a.AttachmentId, EffectTargetSha256: digest, CleanupRevision: 3})
	}
	return &messagingv1.ApplyBotChatPinRemovalResponse{Receipt: r}
}
func TestMessagingDeletionReceiptRejectsCrossTargetAndFalseCompletion(t *testing.T) {
	op, e := messagingDeletionSource(t)
	q, err := buildMessagingDeletionRequest(op, e)
	require.NoError(t, err)
	good := messagingReceipt(t, q)
	require.NoError(t, validateMessagingDeletionReceipt(q, good))
	for _, mode := range []string{"missing target", "wrong digest", "wrong chat", "wrong request", "zero revision", "mixed revision", "false adoption count", "self covering", "unknown target"} {
		t.Run(mode, func(t *testing.T) {
			r := proto.Clone(good).(*messagingv1.ApplyBotChatPinRemovalResponse)
			switch mode {
			case "missing target":
				r.Receipt.Targets = r.Receipt.Targets[:1]
			case "wrong digest":
				r.Receipt.Targets[0].EffectTargetSha256[0] ^= 1
			case "wrong chat":
				r.Receipt.ChatId = uuid.NewString()
			case "wrong request":
				r.Receipt.RequestSha256[0] ^= 1
			case "zero revision":
				r.Receipt.ExecutionCleanupRevision = 0
			case "mixed revision":
				r.Receipt.Targets[0].CleanupRevision++
			case "false adoption count":
				r.Receipt.ExecutionCleanupRevision = 0
				for _, target := range r.Receipt.Targets {
					target.CoveringOperationId = uuid.NewString()
					target.CoveringReceiptId = uuid.NewString()
					target.CoveringResponseSha256 = make([]byte, 32)
				}
			case "self covering":
				r.Receipt.Targets[0].CoveringOperationId = q.OperationId
				r.Receipt.Targets[0].CoveringReceiptId = uuid.NewString()
				r.Receipt.Targets[0].CoveringResponseSha256 = make([]byte, 32)
			case "unknown target":
				r.Receipt.Targets[0].ProtoReflect().SetUnknown([]byte{0xa0, 0x06, 0x01})
			}
			require.Error(t, validateMessagingDeletionReceipt(q, r))
		})
	}
	adopted := proto.Clone(good).(*messagingv1.ApplyBotChatPinRemovalResponse)
	adopted.Receipt.ExecutionCleanupRevision = 0
	adopted.Receipt.RemovedPinCount = 0
	for _, target := range adopted.Receipt.Targets {
		target.CoveringOperationId = uuid.NewString()
		target.CoveringReceiptId = uuid.NewString()
		target.CoveringResponseSha256 = make([]byte, 32)
	}
	require.NoError(t, validateMessagingDeletionReceipt(q, adopted))
}
