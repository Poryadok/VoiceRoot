package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	commonv1 "voice.app/voice/common/v1"
	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/pkg/principal"
)

func notificationHasUnknown(message proto.Message) bool {
	if message == nil {
		return false
	}
	unknown := false
	message.ProtoReflect().Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		if field.IsList() && field.Message() != nil {
			list := value.List()
			for i := 0; i < list.Len(); i++ {
				if notificationHasUnknown(list.Get(i).Message().Interface()) {
					unknown = true
					return false
				}
			}
		} else if field.Message() != nil && notificationHasUnknown(value.Message().Interface()) {
			unknown = true
			return false
		}
		return true
	})
	return unknown || len(message.ProtoReflect().GetUnknown()) != 0
}

func notificationLifecyclePrincipal(ctx context.Context, request proto.Message, rpc string) error {
	verified, ok := principal.FromContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "verified Space principal required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	if verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" ||
		verified.Audience != "notification" || verified.RPC != rpc || verified.RequestID == "" || verified.RequestHash != hash ||
		verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.PermissionDenied, "lifecycle caller is not trusted Space")
	}
	return nil
}

func notificationLifecycleIdentity(spaceID, operationID string, generation uint64) error {
	for field, value := range map[string]string{"space_id": spaceID, "deletion_operation_id": operationID} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return status.Errorf(codes.InvalidArgument, "%s must be a canonical non-zero UUID", field)
		}
	}
	if generation == 0 {
		return status.Error(codes.InvalidArgument, "generation must be positive")
	}
	return nil
}

func validateNotificationFence(request *notificationv1.ApplySpaceLifecycleFenceRequest) error {
	if request == nil || notificationHasUnknown(request) || request.GetFence() == nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle fence")
	}
	fence := request.GetFence()
	if fence.GetProtocolVersion() != 1 || notificationLifecycleIdentity(fence.GetSpaceId(), fence.GetDeletionOperationId(), fence.GetGeneration()) != nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle fence binding")
	}
	if fence.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN &&
		fence.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE &&
		fence.GetDesiredState() != commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED {
		return status.Error(codes.InvalidArgument, "invalid lifecycle fence state")
	}
	manifest := fence.GetManifest()
	if manifest == nil || notificationHasUnknown(manifest) || strings.TrimSpace(manifest.GetManifestId()) == "" || len(manifest.GetManifestSha256()) != sha256.Size {
		return status.Error(codes.InvalidArgument, "invalid lifecycle manifest")
	}
	return nil
}

func (s *NotificationGRPC) ApplySpaceLifecycleFence(ctx context.Context, request *notificationv1.ApplySpaceLifecycleFenceRequest) (*notificationv1.ApplySpaceLifecycleFenceResponse, error) {
	if err := notificationLifecyclePrincipal(ctx, request, notificationv1.NotificationService_ApplySpaceLifecycleFence_FullMethodName); err != nil {
		return nil, err
	}
	if err := validateNotificationFence(request); err != nil {
		return nil, err
	}
	if s == nil || s.SpaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "notification lifecycle store unavailable")
	}
	receipt, err := s.SpaceLifecycle.ApplySpaceLifecycleFence(ctx, request.GetFence())
	if err != nil {
		return nil, status.Error(codes.Unavailable, "notification lifecycle fence unavailable")
	}
	if !validNotificationFenceReceipt(receipt, request.GetFence()) {
		return nil, status.Error(codes.DataLoss, "notification lifecycle fence receipt mismatch")
	}
	return &notificationv1.ApplySpaceLifecycleFenceResponse{Receipt: receipt}, nil
}

func validNotificationFenceReceipt(receipt *commonv1.SpaceLifecycleFenceReceipt, request *commonv1.SpaceLifecycleFenceRequest) bool {
	return receipt != nil && request != nil && receipt.GetProtocolVersion() == 1 && receipt.GetReceiptId() != "" &&
		receipt.GetSpaceId() == request.GetSpaceId() && receipt.GetDeletionOperationId() == request.GetDeletionOperationId() &&
		receipt.GetGeneration() == request.GetGeneration() && receipt.GetParticipantId() == commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION &&
		receipt.GetAppliedState() == request.GetDesiredState() && notificationReceiptMatches(&notificationv1.ApplySpaceLifecycleFenceRequest{Fence: request}, receipt.GetRequestSha256()) &&
		len(receipt.GetManifestSha256()) == sha256.Size && bytes.Equal(receipt.GetManifestSha256(), request.GetManifest().GetManifestSha256()) &&
		receipt.GetAppliedAt() != nil && receipt.GetAppliedAt().CheckValid() == nil
}

func validateNotificationPurge(request *notificationv1.PurgeSpaceRequest) error {
	if request == nil || notificationHasUnknown(request) || request.GetPurge() == nil {
		return status.Error(codes.InvalidArgument, "invalid Space purge")
	}
	purge := request.GetPurge()
	if purge.GetProtocolVersion() != 1 || purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION ||
		notificationLifecycleIdentity(purge.GetSpaceId(), purge.GetDeletionOperationId(), purge.GetGeneration()) != nil ||
		purge.GetPurgeDecidedAt() == nil || purge.GetPurgeDecidedAt().CheckValid() != nil || purge.GetPurgeDecidedAt().GetNanos()%1000 != 0 {
		return status.Error(codes.InvalidArgument, "invalid Space purge binding")
	}
	manifest := purge.GetManifest()
	if manifest == nil || notificationHasUnknown(manifest) || strings.TrimSpace(manifest.GetManifestId()) == "" || len(manifest.GetManifestSha256()) != sha256.Size {
		return status.Error(codes.InvalidArgument, "invalid Space purge manifest")
	}
	return nil
}

func (s *NotificationGRPC) PurgeSpace(ctx context.Context, request *notificationv1.PurgeSpaceRequest) (*notificationv1.PurgeSpaceResponse, error) {
	if err := notificationLifecyclePrincipal(ctx, request, notificationv1.NotificationService_PurgeSpace_FullMethodName); err != nil {
		return nil, err
	}
	if err := validateNotificationPurge(request); err != nil {
		return nil, err
	}
	if s == nil || s.SpaceLifecycle == nil {
		return nil, status.Error(codes.Unavailable, "notification lifecycle store unavailable")
	}
	receipt, err := s.SpaceLifecycle.PurgeSpace(ctx, request.GetPurge())
	if err != nil {
		return nil, status.Error(codes.Unavailable, "notification Space purge unavailable")
	}
	if !validNotificationPurgeReceipt(receipt, request.GetPurge()) {
		return nil, status.Error(codes.DataLoss, "notification Space purge receipt mismatch")
	}
	return &notificationv1.PurgeSpaceResponse{Receipt: receipt}, nil
}

func validNotificationPurgeReceipt(receipt *commonv1.SpacePurgeReceipt, request *commonv1.SpacePurgeRequest) bool {
	return receipt != nil && request != nil && receipt.GetProtocolVersion() == 1 && receipt.GetReceiptId() != "" &&
		receipt.GetSpaceId() == request.GetSpaceId() && receipt.GetDeletionOperationId() == request.GetDeletionOperationId() &&
		receipt.GetGeneration() == request.GetGeneration() && receipt.GetParticipantId() == commonv1.ParticipantId_PARTICIPANT_ID_NOTIFICATION &&
		receipt.GetState() == commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED &&
		notificationReceiptMatches(&notificationv1.PurgeSpaceRequest{Purge: request}, receipt.GetRequestSha256()) && receipt.GetCompletedAt() != nil && receipt.GetCompletedAt().CheckValid() == nil
}

func notificationReceiptMatches(request proto.Message, digest []byte) bool {
	if len(digest) != sha256.Size {
		return false
	}
	encoded, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return false
	}
	expected := sha256.Sum256(append(append([]byte(request.ProtoReflect().Descriptor().FullName()), 0), encoded...))
	return bytes.Equal(expected[:], digest)
}
