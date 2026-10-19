package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func buildSpaceDeletionRequest(claim *store.BotDeletion, effect store.BotDeletionEffect) (*spacev1.ApplyBotInstallationRemovalRequest, error) {
	if cleanupPurpose(claim) != "global_delete" {
		return nil, errDeletionEvidence
	}
	if effect.Kind != "SPACE_CLEANUP" || effect.ChatID != nil || effect.Target.CancelOperationID == uuid.Nil {
		return nil, errDeletionEvidence
	}
	roleEffect := effect
	roleEffect.Kind = "ROLE_REMOVE"
	root, err := buildRoleDeletionRequest(claim, roleEffect)
	if err != nil {
		return nil, err
	}
	native := root.GetNative()
	if native == nil {
		return nil, errDeletionEvidence
	}
	cancel := &spacev1.RemoveBotMemberRequest{SpaceId: root.SpaceId, ProfileId: root.ActorProfileId, InstallOperationId: native.InstallOperationId, InstallationIntent: &spacev1.BotInstallationIntent{OperationId: effect.Target.CancelOperationID.String(), BotId: root.BotId, InstallationId: native.InstallationId, SpaceId: root.SpaceId, ActorProfileId: root.ActorProfileId, Generation: native.Generation}}
	return &spacev1.ApplyBotInstallationRemovalRequest{ProtocolVersion: 1, OperationId: root.OperationId, DecisionId: root.DecisionId, DecisionSha256: root.DecisionSha256, Purpose: root.Purpose, BotId: root.BotId, ActorProfileId: root.ActorProfileId, SpaceId: root.SpaceId, NativeCancellation: cancel, FrozenScopeSha256: root.FrozenScopeSha256}, nil
}

func validateSpaceDeletionReceipt(request *spacev1.ApplyBotInstallationRemovalRequest, response *spacev1.ApplyBotInstallationRemovalResponse) error {
	if request == nil || request.NativeCancellation == nil || response == nil || len(response.ProtoReflect().GetUnknown()) != 0 || response.Receipt == nil {
		return errDeletionEvidence
	}
	r := response.Receipt
	if len(r.ProtoReflect().GetUnknown()) != 0 || r.ProtocolVersion != 1 || !deletionUUID(r.ReceiptId) || r.OperationId != request.OperationId || r.DecisionId != request.DecisionId || r.Purpose != "global_delete" || r.Purpose != request.Purpose || r.BotId != request.BotId || r.ActorProfileId != request.ActorProfileId || r.SpaceId != request.SpaceId || r.EvidenceKind != "native" || r.Participant != "space" || r.Outcome != "COMPLETED" || !r.GlobalNativeAddsFenced || !bytes.Equal(r.FrozenScopeSha256, request.FrozenScopeSha256) || r.CompletedAt == nil || r.CompletedAt.CheckValid() != nil || r.CompletedAt.Seconds <= 0 || len(r.CompletedAt.ProtoReflect().GetUnknown()) != 0 {
		return errDeletionEvidence
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return errDeletionEvidence
	}
	requestSHA := sha256.Sum256(body)
	nested, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request.NativeCancellation)
	if err != nil {
		return errDeletionEvidence
	}
	evidenceSHA := sha256.Sum256(nested)
	if !bytes.Equal(requestSHA[:], r.RequestSha256) || !bytes.Equal(evidenceSHA[:], r.EvidenceSha256) {
		return errDeletionEvidence
	}
	return nil
}

func (s *BotGRPC) applySpaceDeletion(ctx context.Context, claim *store.BotDeletion, effect store.BotDeletionEffect) error {
	if s == nil || s.Store == nil || s.CleanupSpace == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "protected global Space cleanup authority unavailable")
	}
	request, err := buildSpaceDeletionRequest(claim, effect)
	if err != nil {
		return status.Error(codes.Unavailable, "frozen Space cleanup provenance is not qualified")
	}
	body, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return err
	}
	if len(effect.RequestBytes) != 0 && !bytes.Equal(effect.RequestBytes, body) {
		return store.ErrInstallationConflict
	}
	if err = s.Store.SealBotDeletionRequest(ctx, claim, effect.EffectID, body); err != nil {
		return err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return err
	}
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ApplyBotInstallationRemoval_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Bot cleanup principal unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
	response, err := s.CleanupSpace.ApplyBotInstallationRemoval(callCtx, request)
	if err != nil {
		return err
	}
	if validateSpaceDeletionReceipt(request, response) != nil {
		return status.Error(codes.Unavailable, "Space global cleanup acknowledgement is not qualified")
	}
	receipt, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return err
	}
	return s.Store.SaveBotDeletionReceipt(ctx, claim, effect.EffectID, body, receipt)
}
