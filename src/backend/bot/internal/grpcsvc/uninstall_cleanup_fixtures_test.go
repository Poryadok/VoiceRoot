package grpcsvc_test

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	messagingv1 "voice.app/voice/messaging/v1"
	rolev1 "voice.app/voice/role/v1"
)

// Controlled typed participant replies keep existing Bot semantic tests focused
// on their original assertions. They are not protected receiver/runtime proof.
func fixtureUninstallRoleReceipt(q *rolev1.ApplyBotInstallationRemovalRequest) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	if q.Purpose != "space_uninstall" || q.GetNative() == nil {
		return nil, errors.New("fixture requires exact native uninstall")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	evidence, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q.GetNative())
	if err != nil {
		return nil, err
	}
	evidenceSum := sha256.Sum256(evidence)
	return &rolev1.ApplyBotInstallationRemovalResponse{Receipt: &rolev1.BotInstallationRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, EvidenceKind: "native", EvidenceSha256: evidenceSum[:], FrozenScopeSha256: q.FrozenScopeSha256, RequestSha256: sum[:], Participant: "role", Outcome: "COMPLETED", CompletedAt: timestamppb.Now()}}, nil
}

type fixtureUninstallRoleClient struct{ rolev1.RoleServiceClient }

func (fixtureUninstallRoleClient) ApplyBotInstallationRemoval(_ context.Context, q *rolev1.ApplyBotInstallationRemovalRequest, _ ...grpc.CallOption) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	return fixtureUninstallRoleReceipt(q)
}
func (*fakeRoleClient) ApplyBotInstallationRemoval(_ context.Context, q *rolev1.ApplyBotInstallationRemovalRequest) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	return fixtureUninstallRoleReceipt(q)
}

func fixtureUninstallMessagingReceipt(q *messagingv1.ApplyBotChatPinRemovalRequest) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	if q.Purpose != "space_uninstall" || q.GetNative() == nil || len(q.Attachments) == 0 {
		return nil, errors.New("fixture requires native attachment cleanup")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(body)
	evidence, err := (proto.MarshalOptions{Deterministic: true}).Marshal(q.GetNative())
	if err != nil {
		return nil, err
	}
	evidenceSum := sha256.Sum256(evidence)
	r := &messagingv1.BotChatPinRemovalReceipt{ProtocolVersion: 1, ReceiptId: uuid.NewString(), OperationId: q.OperationId, DecisionId: q.DecisionId, Purpose: q.Purpose, BotId: q.BotId, ActorProfileId: q.ActorProfileId, SpaceId: q.SpaceId, ChatId: q.ChatId, EvidenceKind: "native", EvidenceSha256: evidenceSum[:], FrozenScopeSha256: q.FrozenScopeSha256, RequestSha256: sum[:], Participant: "messaging", Outcome: "COMPLETED", CompletedAt: timestamppb.Now(), ExecutionCleanupRevision: 1}
	// c49's projection is used only to construct a controlled matching response;
	// separate manual golden and real Messaging tests own codec/effect validation.
	for _, a := range q.Attachments {
		projection := proto.Clone(q).(*messagingv1.ApplyBotChatPinRemovalRequest)
		projection.OperationId, projection.DecisionId, projection.Purpose = "", "", ""
		projection.DecisionSha256, projection.FrozenScopeSha256 = nil, nil
		one := proto.Clone(a).(*messagingv1.BotChatPinRemovalAttachment)
		one.CurrentScope = nil
		one.CoveringOperationId = ""
		projection.Attachments = []*messagingv1.BotChatPinRemovalAttachment{one}
		body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(projection)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(append([]byte("voice.bot-pin-attachment.v1\x00"), body...))
		r.Targets = append(r.Targets, &messagingv1.BotChatPinRemovalTargetReceipt{AttachmentId: a.AttachmentId, EffectTargetSha256: digest[:], CleanupRevision: 1})
	}
	return &messagingv1.ApplyBotChatPinRemovalResponse{Receipt: r}, nil
}

type fixtureUninstallMessagingClient struct {
	messagingv1.MessagingServiceClient
}

func (fixtureUninstallMessagingClient) ApplyBotChatPinRemoval(_ context.Context, q *messagingv1.ApplyBotChatPinRemovalRequest, _ ...grpc.CallOption) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	return fixtureUninstallMessagingReceipt(q)
}
func (*fakeMessagingClient) ApplyBotChatPinRemoval(_ context.Context, q *messagingv1.ApplyBotChatPinRemovalRequest) (*messagingv1.ApplyBotChatPinRemovalResponse, error) {
	return fixtureUninstallMessagingReceipt(q)
}
