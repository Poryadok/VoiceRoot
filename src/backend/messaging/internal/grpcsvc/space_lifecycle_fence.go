package grpcsvc

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/messaging/internal/store"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type SpaceLifecycleFenceApplier interface {
	ApplySpaceLifecycleFence(context.Context, store.SpaceLifecycleFenceInput) ([]byte, error)
}

func (s *MessagingGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *messagingv1.ApplySpaceLifecycleFenceRequest) (*messagingv1.ApplySpaceLifecycleFenceResponse, error) {
	if req == nil || hasAnyUnknown(req) {
		return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle fence")
	}
	if err := trustedMessagingSpaceLifecycle(ctx, req, messagingv1.MessagingService_ApplySpaceLifecycleFence_FullMethodName); err != nil {
		return nil, err
	}
	fence := req.GetFence()
	if fence == nil || fence.GetProtocolVersion() != 1 || fence.GetGeneration() == 0 || fence.GetManifest() == nil ||
		fence.GetManifest().GetManifestId() == "" || len(fence.GetManifest().GetManifestSha256()) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle fence")
	}
	spaceID, err := canonicalSpaceLifecycleUUID(fence.GetSpaceId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle fence")
	}
	operationID, err := canonicalSpaceLifecycleUUID(fence.GetDeletionOperationId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle fence")
	}
	if s == nil || s.SpaceLifecycleFences == nil {
		return nil, status.Error(codes.Unavailable, "Messaging Space lifecycle store unavailable")
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Space lifecycle fence")
	}
	requestHash := domainSHA(string(req.ProtoReflect().Descriptor().FullName()), requestBytes)
	receipt := &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion:     1,
		ReceiptId:           uuid.NewString(),
		SpaceId:             spaceID.String(),
		DeletionOperationId: operationID.String(),
		Generation:          fence.GetGeneration(),
		ParticipantId:       commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING,
		AppliedState:        fence.GetDesiredState(),
		RequestSha256:       requestHash,
		ManifestSha256:      bytes.Clone(fence.GetManifest().GetManifestSha256()),
		AppliedAt:           timestamppb.Now(),
	}
	receiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(&messagingv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt})
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot encode Messaging lifecycle receipt")
	}
	savedBytes, err := s.SpaceLifecycleFences.ApplySpaceLifecycleFence(ctx, store.SpaceLifecycleFenceInput{
		SpaceID:             spaceID,
		DeletionOperationID: operationID,
		Generation:          fence.GetGeneration(),
		State:               fence.GetDesiredState(),
		Manifest:            fence.GetManifest(),
		RequestBytes:        requestBytes,
		RequestSHA256:       requestHash,
		ReceiptBytes:        receiptBytes,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrSpaceManifestNotSealed):
			return nil, status.Error(codes.FailedPrecondition, "Messaging manifest is not sealed")
		case errors.Is(err, store.ErrSpaceManifestBinding), errors.Is(err, store.ErrSpaceLifecycleConflict), errors.Is(err, store.ErrSpaceLifecycleOrder):
			return nil, status.Error(codes.FailedPrecondition, "Messaging lifecycle fence conflicts with saved evidence")
		default:
			return nil, status.Error(codes.Unavailable, "Messaging Space lifecycle store unavailable")
		}
	}
	saved := &messagingv1.ApplySpaceLifecycleFenceResponse{}
	if err := proto.Unmarshal(savedBytes, saved); err != nil || !validSpaceLifecycleFenceReceipt(saved.GetReceipt(), fence, requestHash) {
		return nil, status.Error(codes.Internal, "stored Messaging lifecycle receipt is invalid")
	}
	if fence.DesiredState==commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN {
		if s.SpaceFileProducer==nil { return nil,status.Error(codes.Unavailable,"Messaging File producer unavailable") }
		if err:=s.SpaceFileProducer.Seal(ctx,fence); err!=nil { return nil,err }
	}
	return saved, nil
}

func validSpaceLifecycleFenceReceipt(receipt *commonv1.SpaceLifecycleFenceReceipt, fence *commonv1.SpaceLifecycleFenceRequest, requestHash []byte) bool {
	if receipt == nil || receipt.GetProtocolVersion() != 1 || receipt.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING ||
		receipt.GetSpaceId() != fence.GetSpaceId() || len(receipt.GetRequestSha256()) != 32 || len(receipt.GetManifestSha256()) != 32 ||
		!bytes.Equal(receipt.GetManifestSha256(), fence.GetManifest().GetManifestSha256()) || receipt.GetGeneration() == 0 ||
		receipt.GetAppliedAt() == nil || receipt.GetAppliedAt().CheckValid() != nil || receipt.GetAppliedAt().AsTime().Unix() <= 0 {
		return false
	}
	if receipt.GetGeneration() < fence.GetGeneration() {
		return true
	}
	return receipt.GetGeneration() == fence.GetGeneration() && receipt.GetDeletionOperationId() == fence.GetDeletionOperationId() &&
		receipt.GetAppliedState() == fence.GetDesiredState() && bytes.Equal(receipt.GetRequestSha256(), requestHash)
}
