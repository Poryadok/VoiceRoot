package grpcsvc

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"voice/backend/pkg/principal"
	"voice/backend/subscription/internal/store"

	commonv1 "voice.app/voice/common/v1"
	subscriptionv1 "voice.app/voice/subscription/v1"
)

func requireSpaceLifecyclePrincipal(ctx context.Context, rpc string, request proto.Message) error {
	verified, ok := principal.FromContext(ctx)
	if !ok || verified.Kind != "service" || verified.Issuer != "space" || verified.Subject != "service:space" ||
		verified.Audience != "subscription" || verified.RPC != rpc || strings.TrimSpace(verified.RequestID) == "" {
		return status.Error(codes.PermissionDenied, "verified space principal required")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid lifecycle request")
	}
	if verified.RequestHash != hash {
		return status.Error(codes.PermissionDenied, "verified lifecycle request binding required")
	}
	return nil
}

func lifecycleDomainHash(message proto.Message) ([]byte, []byte, error) {
	wire, err := (proto.MarshalOptions{Deterministic: true}).Marshal(message)
	if err != nil {
		return nil, nil, err
	}
	h := sha256.New()
	_, _ = h.Write([]byte(message.ProtoReflect().Descriptor().FullName()))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write(wire)
	return h.Sum(nil), wire, nil
}

func validateManifest(manifest *commonv1.ManifestBinding) error {
	if manifest == nil || strings.TrimSpace(manifest.GetManifestId()) == "" || len(manifest.GetManifestSha256()) != sha256.Size {
		return status.Error(codes.InvalidArgument, "complete manifest binding required")
	}
	return nil
}

func lifecycleStateName(state commonv1.LifecycleFenceState) (string, error) {
	switch state {
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN:
		return "FROZEN", nil
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE:
		return "LIVE", nil
	case commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED:
		return "PURGE_DECIDED", nil
	default:
		return "", status.Error(codes.InvalidArgument, "valid lifecycle state required")
	}
}

func lifecycleStateProto(state string) commonv1.LifecycleFenceState {
	switch state {
	case "FROZEN":
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN
	case "LIVE":
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE
	case "PURGE_DECIDED", "PURGED":
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_PURGE_DECIDED
	default:
		return commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_UNSPECIFIED
	}
}

func lifecycleStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrLifecycleBinding):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, store.ErrLifecycleGeneration), errors.Is(err, store.ErrLifecycleState):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, store.ErrProviderCancellation):
		return status.Error(codes.Unavailable, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}

func (s *SubscriptionGRPC) ApplySpaceLifecycleFence(ctx context.Context, req *subscriptionv1.ApplySpaceLifecycleFenceRequest) (*subscriptionv1.ApplySpaceLifecycleFenceResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, subscriptionv1.SubscriptionService_ApplySpaceLifecycleFence_FullMethodName, req); err != nil {
		return nil, err
	}
	if req == nil || req.GetFence() == nil {
		return nil, status.Error(codes.InvalidArgument, "lifecycle fence required")
	}
	fence := req.GetFence()
	if fence.GetProtocolVersion() != 1 || fence.GetGeneration() == 0 {
		return nil, status.Error(codes.InvalidArgument, "supported protocol and positive generation required")
	}
	spaceID, err := parseUUIDField("space_id", fence.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUIDField("deletion_operation_id", fence.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	if err := validateManifest(fence.GetManifest()); err != nil {
		return nil, err
	}
	state, err := lifecycleStateName(fence.GetDesiredState())
	if err != nil {
		return nil, err
	}
	requestHash, requestBytes, err := lifecycleDomainHash(fence)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle fence")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "subscription lifecycle store not configured")
	}
	appliedAt := time.Now().UTC()
	receiptID := uuid.New()
	response := &subscriptionv1.ApplySpaceLifecycleFenceResponse{Receipt: &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(),
		Generation: fence.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		AppliedState: fence.GetDesiredState(), RequestSha256: requestHash,
		ManifestSha256: append([]byte(nil), fence.GetManifest().GetManifestSha256()...), AppliedAt: timestamppb.New(appliedAt),
	}}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot encode lifecycle receipt")
	}
	record, err := s.Store.ApplySpaceLifecycleFence(ctx, store.LifecycleFenceInput{
		SpaceID: spaceID, DeletionOperationID: deletionID, Generation: int64(fence.GetGeneration()), State: state,
		ManifestID: fence.GetManifest().GetManifestId(), ManifestSHA256: fence.GetManifest().GetManifestSha256(),
		ManifestItemCount: int64(fence.GetManifest().GetItemCount()), RequestSHA256: requestHash, RequestBytes: requestBytes,
		ReceiptID: receiptID, ReceiptBytes: receiptBytes, AppliedAt: appliedAt,
	})
	if err != nil {
		return nil, lifecycleStoreError(err)
	}
	if len(record.ReceiptBytes) > 0 {
		stored := &subscriptionv1.ApplySpaceLifecycleFenceResponse{}
		if err := proto.Unmarshal(record.ReceiptBytes, stored); err != nil {
			return nil, status.Error(codes.Internal, "stored lifecycle receipt is invalid")
		}
		return stored, nil
	}
	return &subscriptionv1.ApplySpaceLifecycleFenceResponse{Receipt: &commonv1.SpaceLifecycleFenceReceipt{
		ProtocolVersion: 1, ReceiptId: record.ReceiptID.String(), SpaceId: record.SpaceID.String(), DeletionOperationId: record.DeletionOperationID.String(),
		Generation: uint64(record.Generation), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		AppliedState: lifecycleStateProto(record.State), RequestSha256: append([]byte(nil), record.RequestSHA256...),
		ManifestSha256: append([]byte(nil), record.ManifestSHA256...), AppliedAt: timestamppb.New(record.AppliedAt),
	}}, nil
}

func (s *SubscriptionGRPC) PurgeSpace(ctx context.Context, req *subscriptionv1.PurgeSpaceRequest) (*subscriptionv1.PurgeSpaceResponse, error) {
	if err := requireSpaceLifecyclePrincipal(ctx, subscriptionv1.SubscriptionService_PurgeSpace_FullMethodName, req); err != nil {
		return nil, err
	}
	if req == nil || req.GetPurge() == nil {
		return nil, status.Error(codes.InvalidArgument, "space purge request required")
	}
	purge := req.GetPurge()
	if purge.GetProtocolVersion() != 1 || purge.GetGeneration() == 0 || purge.GetParticipantId() != commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION ||
		purge.GetPurgeDecidedAt() == nil || !purge.GetPurgeDecidedAt().IsValid() {
		return nil, status.Error(codes.InvalidArgument, "valid subscription purge binding required")
	}
	spaceID, err := parseUUIDField("space_id", purge.GetSpaceId())
	if err != nil {
		return nil, err
	}
	deletionID, err := parseUUIDField("deletion_operation_id", purge.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	if err := validateManifest(purge.GetManifest()); err != nil {
		return nil, err
	}
	requestHash, requestBytes, err := lifecycleDomainHash(purge)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid purge request")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.FailedPrecondition, "subscription lifecycle store not configured")
	}
	completedAt := time.Now().UTC()
	receiptID := uuid.New()
	response := &subscriptionv1.PurgeSpaceResponse{Receipt: &commonv1.SpacePurgeReceipt{
		ProtocolVersion: 1, ReceiptId: receiptID.String(), SpaceId: spaceID.String(), DeletionOperationId: deletionID.String(),
		Generation: purge.GetGeneration(), ParticipantId: commonv1.ParticipantId_PARTICIPANT_ID_SUBSCRIPTION,
		State: commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED, RequestSha256: requestHash, CompletedAt: timestamppb.New(completedAt),
	}}
	receiptBytes, err := (proto.MarshalOptions{Deterministic: true}).Marshal(response)
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot encode purge receipt")
	}
	var cancel store.RenewalCanceller
	if s.ProviderRenewals != nil {
		cancel = s.ProviderRenewals.CancelSpaceRenewal
	}
	storedBytes, err := s.Store.PurgeSpace(ctx, store.PurgeInput{
		SpaceID: spaceID, DeletionOperationID: deletionID, Generation: int64(purge.GetGeneration()),
		ManifestID: purge.GetManifest().GetManifestId(), ManifestSHA256: purge.GetManifest().GetManifestSha256(),
		ManifestItemCount: int64(purge.GetManifest().GetItemCount()), RequestSHA256: requestHash, RequestBytes: requestBytes,
		ReceiptID: receiptID, ReceiptBytes: receiptBytes, CompletedAt: completedAt,
	}, cancel)
	if err != nil {
		return nil, lifecycleStoreError(err)
	}
	stored := &subscriptionv1.PurgeSpaceResponse{}
	if err := proto.Unmarshal(storedBytes, stored); err != nil {
		return nil, status.Error(codes.Internal, "stored purge receipt is invalid")
	}
	return stored, nil
}
