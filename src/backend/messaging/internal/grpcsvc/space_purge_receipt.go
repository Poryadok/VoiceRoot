package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"voice/backend/messaging/internal/store"
	"voice/backend/pkg/principal"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// SpacePurgeReceiptLookup returns only an immutable committed Messaging
// participant receipt. It has no mutation or purge-start capability.
type SpacePurgeReceiptLookup interface {
	GetSpacePurgeReceipt(context.Context, store.SpacePurgeReceiptKey) (*commonv1.SpacePurgeReceipt, error)
}

func (s *MessagingGRPC) GetSpacePurgeReceipt(ctx context.Context, req *messagingv1.GetSpacePurgeReceiptRequest) (*messagingv1.GetSpacePurgeReceiptResponse, error) {
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid purge receipt lookup")
	}
	verified, ok := principal.FromContext(ctx)
	hash, hashErr := principal.RequestHash(req)
	if !ok || hashErr != nil || verified.Kind != "service" || verified.Issuer != "chat" ||
		verified.Subject != "service:chat" || verified.Audience != "messaging" ||
		verified.RPC != messagingv1.MessagingService_GetSpacePurgeReceipt_FullMethodName ||
		strings.TrimSpace(verified.RequestID) == "" || verified.RequestHash != hash {
		return nil, status.Error(codes.Unauthenticated, "invalid Chat principal binding")
	}
	spaceID, err := canonicalSpaceLifecycleUUID(req.GetSpaceId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid purge receipt lookup")
	}
	deletionID, err := canonicalSpaceLifecycleUUID(req.GetDeletionOperationId())
	if err != nil || req.GetPurgeGeneration() == 0 || req.GetSourceScheduleGeneration() == 0 ||
		req.GetSourceScheduleGeneration()+1 != req.GetPurgeGeneration() || len(req.GetMessagingRequestSha256()) != 32 {
		return nil, status.Error(codes.InvalidArgument, "invalid purge receipt lookup")
	}
	if s == nil || s.SpacePurgeReceipts == nil {
		return nil, status.Error(codes.Unavailable, "Messaging purge receipt lookup unavailable")
	}
	receipt, err := s.SpacePurgeReceipts.GetSpacePurgeReceipt(ctx, store.SpacePurgeReceiptKey{
		SpaceID: spaceID, DeletionOperationID: deletionID, PurgeGeneration: req.GetPurgeGeneration(),
		SourceScheduleGeneration: req.GetSourceScheduleGeneration(), MessagingRequestSHA256: append([]byte(nil), req.GetMessagingRequestSha256()...),
	})
	if err != nil {
		if errors.Is(err, store.ErrSpacePurgeReceiptNotFound) {
			return nil, status.Error(codes.NotFound, "committed Messaging purge receipt not found")
		}
		if errors.Is(err, store.ErrSpacePurgeReceiptBinding) {
			return nil, status.Error(codes.FailedPrecondition, "Messaging purge receipt binding mismatch")
		}
		return nil, status.Error(codes.Unavailable, "Messaging purge receipt lookup unavailable")
	}
	if !validMessagingPurgeReceipt(receipt, req) {
		return nil, status.Error(codes.Unavailable, "stored Messaging purge receipt is invalid")
	}
	return &messagingv1.GetSpacePurgeReceiptResponse{Receipt: receipt}, nil
}

func canonicalSpaceLifecycleUUID(value string) (uuid.UUID, error) {
	id, err := uuid.Parse(value)
	if err != nil || id == uuid.Nil || id.String() != value {
		return uuid.Nil, errors.New("invalid canonical UUID")
	}
	return id, nil
}

func validMessagingPurgeReceipt(receipt *commonv1.SpacePurgeReceipt, req *messagingv1.GetSpacePurgeReceiptRequest) bool {
	return receipt != nil && receipt.GetProtocolVersion() == 1 && receipt.GetSpaceId() == req.GetSpaceId() &&
		receipt.GetDeletionOperationId() == req.GetDeletionOperationId() && receipt.GetGeneration() == req.GetPurgeGeneration() &&
		receipt.GetParticipantId() == commonv1.ParticipantId_PARTICIPANT_ID_MESSAGING &&
		receipt.GetState() == commonv1.PurgeReceiptState_PURGE_RECEIPT_STATE_COMPLETED &&
		len(receipt.GetRequestSha256()) == 32 && string(receipt.GetRequestSha256()) == string(req.GetMessagingRequestSha256()) &&
		receipt.GetCompletedAt() != nil && receipt.GetCompletedAt().CheckValid() == nil &&
		receipt.GetCompletedAt().AsTime().Unix() > 0
}
