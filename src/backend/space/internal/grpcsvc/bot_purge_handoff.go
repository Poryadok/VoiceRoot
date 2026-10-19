package grpcsvc

import (
	"bytes"
	"context"
	"errors"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
	"voice/backend/space/internal/store"
)

type BotPurgeHandoffStore interface {
	GetBotPurgeHandoff(context.Context, uuid.UUID, uuid.UUID, uint64, *commonv1.ManifestBinding) (*store.BotPurgeHandoff, error)
}

func (s *SpaceGRPC) GetBotPurgeHandoff(context.Context, *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
}

func (s *BotMembershipGRPC) GetBotPurgeHandoff(ctx context.Context, request *spacev1.GetBotPurgeHandoffRequest) (*spacev1.GetBotPurgeHandoffResponse, error) {
	proof, err := botmembershipprincipal.RequirePurgeHandoff(ctx, request)
	if err != nil {
		return nil, err
	}
	r := proto.Clone(request).(*spacev1.GetBotPurgeHandoffRequest)
	binding, err := proof.Check(r)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Bot purge handoff unavailable")
	}
	reader, ok := s.Store.(BotPurgeHandoffStore)
	if !ok {
		return nil, status.Error(codes.Unavailable, "Bot purge handoff unavailable")
	}
	got, err := reader.GetBotPurgeHandoff(ctx, binding.SpaceID, binding.DeletionOperationID, binding.Generation, r.Manifest)
	if err != nil {
		if errors.Is(err, store.ErrLifecycleConflict) || errors.Is(err, store.ErrLifecycleStateTransition) {
			return nil, status.Error(codes.FailedPrecondition, "Bot purge handoff binding is not eligible")
		}
		return nil, status.Error(codes.Unavailable, "Bot purge handoff evidence unavailable")
	}
	if got == nil {
		return nil, status.Error(codes.Unavailable, "Bot purge handoff evidence unavailable")
	}
	result := &spacev1.GetBotPurgeHandoffResponse{ProtocolVersion: 1, RequestId: r.RequestId, SpaceId: r.SpaceId, DeletionOperationId: r.DeletionOperationId, Generation: r.Generation, Manifest: proto.Clone(r.Manifest).(*commonv1.ManifestBinding)}
	if got.Role != nil {
		result.Role = &spacev1.BotPurgeRoleHandoff{OriginalReceipt: got.Role.Receipt, OriginalRequestBytes: bytes.Clone(got.Role.RequestBytes), OriginalRequestSha256: bytes.Clone(got.Role.RequestSHA256), OriginalReceiptBytes: bytes.Clone(got.Role.ReceiptBytes), OriginalReceiptSha256: bytes.Clone(got.Role.ReceiptSHA256)}
	}
	if got.Messaging != nil {
		result.Messaging = &spacev1.BotPurgeMessagingHandoff{OriginalReceipt: got.Messaging.Receipt, OriginalRequestBytes: bytes.Clone(got.Messaging.RequestBytes), OriginalRequestSha256: bytes.Clone(got.Messaging.RequestSHA256), OriginalReceiptBytes: bytes.Clone(got.Messaging.ReceiptBytes), OriginalReceiptSha256: bytes.Clone(got.Messaging.ReceiptSHA256)}
	}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	return result, nil
}
