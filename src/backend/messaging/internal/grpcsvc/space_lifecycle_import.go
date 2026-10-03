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
	"voice/backend/pkg/principal"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type SpaceManifestPageImporter interface {
	ImportSpacePurgeManifestPage(context.Context, store.SpacePurgeManifestPageInput) ([]byte, error)
}

func (s *MessagingGRPC) ImportSpacePurgeManifestPage(ctx context.Context, req *messagingv1.ImportSpacePurgeManifestPageRequest) (*messagingv1.ImportSpacePurgeManifestPageResponse, error) {
	if err := validateSpacePurgeManifestPageRequest(req); err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Space purge manifest page")
	}
	if err := trustedMessagingSpaceLifecycle(ctx, req, messagingv1.MessagingService_ImportSpacePurgeManifestPage_FullMethodName); err != nil {
		return nil, err
	}
	if s == nil || s.SpaceManifestImporter == nil {
		return nil, status.Error(codes.Unavailable, "Messaging Space manifest import unavailable")
	}
	requestBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid Space purge manifest page")
	}
	requestHash := domainSHA(string(req.ProtoReflect().Descriptor().FullName()), requestBytes)
	spaceID, _ := canonicalSpaceLifecycleUUID(req.GetSpaceId())
	deletionID, _ := canonicalSpaceLifecycleUUID(req.GetDeletionOperationId())
	page := req.GetPage()
	response := &messagingv1.ImportSpacePurgeManifestPageResponse{Receipt: &messagingv1.ImportSpacePurgeManifestPageReceipt{
		ProtocolVersion:     1,
		ReceiptId:           uuid.NewString(),
		SpaceId:             spaceID.String(),
		DeletionOperationId: deletionID.String(),
		Generation:          req.GetScheduleGeneration(),
		Manifest:            proto.Clone(page.GetManifest()).(*commonv1.ManifestBinding),
		PageIndex:           page.GetPageIndex(),
		AcceptedCount:       uint64(len(page.GetItemIds())),
		PageSha256:          append([]byte(nil), page.GetPageSha256()...),
		ManifestSealed:      req.GetSealsManifest(),
		RequestSha256:       requestHash,
		CompletedAt:         timestamppb.Now(),
	}}
	receiptBytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(response)
	if err != nil {
		return nil, status.Error(codes.Internal, "cannot encode Space manifest receipt")
	}
	savedBytes, err := s.SpaceManifestImporter.ImportSpacePurgeManifestPage(ctx, store.SpacePurgeManifestPageInput{
		SpaceID:             spaceID,
		DeletionOperationID: deletionID,
		ScheduleGeneration:  req.GetScheduleGeneration(),
		Page:                proto.Clone(page).(*chatv1.SpacePurgeManifestPage),
		SealsManifest:       req.GetSealsManifest(),
		RequestBytes:        requestBytes,
		RequestSHA256:       requestHash,
		ReceiptBytes:        receiptBytes,
	})
	if err != nil {
		switch {
		case errors.Is(err, store.ErrSpaceManifestBinding), errors.Is(err, store.ErrSpaceManifestOrder):
			return nil, status.Error(codes.FailedPrecondition, "Space manifest page conflicts with saved evidence")
		default:
			return nil, status.Error(codes.Unavailable, "Messaging Space manifest import unavailable")
		}
	}
	saved := &messagingv1.ImportSpacePurgeManifestPageResponse{}
	if err := proto.Unmarshal(savedBytes, saved); err != nil || !validSpaceManifestImportReceipt(saved.GetReceipt(), req, requestHash) {
		return nil, status.Error(codes.Internal, "stored Space manifest receipt is invalid")
	}
	return saved, nil
}

func validSpaceManifestImportReceipt(receipt *messagingv1.ImportSpacePurgeManifestPageReceipt, req *messagingv1.ImportSpacePurgeManifestPageRequest, requestHash []byte) bool {
	if receipt == nil || receipt.GetProtocolVersion() != 1 || receipt.GetSpaceId() != req.GetSpaceId() ||
		receipt.GetDeletionOperationId() != req.GetDeletionOperationId() || receipt.GetGeneration() != req.GetScheduleGeneration() ||
		!proto.Equal(receipt.GetManifest(), req.GetPage().GetManifest()) || receipt.GetPageIndex() != req.GetPage().GetPageIndex() ||
		receipt.GetAcceptedCount() != uint64(len(req.GetPage().GetItemIds())) ||
		!bytes.Equal(receipt.GetPageSha256(), req.GetPage().GetPageSha256()) || receipt.GetManifestSealed() != req.GetSealsManifest() ||
		!bytes.Equal(receipt.GetRequestSha256(), requestHash) || receipt.GetCompletedAt() == nil || receipt.GetCompletedAt().CheckValid() != nil ||
		receipt.GetCompletedAt().AsTime().Unix() <= 0 {
		return false
	}
	receiptID, err := uuid.Parse(receipt.GetReceiptId())
	return err == nil && receiptID != uuid.Nil && receiptID.String() == receipt.GetReceiptId()
}

func trustedMessagingSpaceLifecycle(ctx context.Context, message proto.Message, rpc string) error {
	verified, ok := principal.FromContext(ctx)
	hash, hashErr := principal.RequestHash(message)
	if !ok || hashErr != nil || verified.Kind != "service" || verified.Issuer != "space" ||
		verified.Subject != "service:space" || verified.Audience != "messaging" || verified.RPC != rpc ||
		verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "invalid Space principal binding")
	}
	return nil
}
