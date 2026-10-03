package grpcsvc

import (
	"bytes"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	notificationv1 "voice.app/voice/notification/v1"
)

type notificationManifestImporter interface {
	ImportSpacePurgeManifestPage(context.Context, *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageReceipt, error)
}

func (s *NotificationGRPC) ImportSpacePurgeManifestPage(ctx context.Context, request *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageResponse, error) {
	if err := notificationLifecyclePrincipal(ctx, request, notificationv1.NotificationService_ImportSpacePurgeManifestPage_FullMethodName); err != nil {
		return nil, err
	}
	// The imported Chat page explicitly accepts/preserves its own unknown
	// fields. The wrapper and root binding reject unknown fields.
	if request == nil || len(request.ProtoReflect().GetUnknown()) != 0 || request.GetProtocolVersion() != 1 || request.GetPage() == nil || request.GetPage().GetManifest() == nil || notificationHasUnknown(request.GetPage().GetManifest()) || notificationLifecycleIdentity(request.GetSpaceId(), request.GetDeletionOperationId(), request.GetScheduleGeneration()) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid manifest import")
	}
	if s == nil {
		return nil, status.Error(codes.Unavailable, "notification manifest store unavailable")
	}
	importer, ok := s.SpaceLifecycle.(notificationManifestImporter)
	if !ok {
		return nil, status.Error(codes.Unavailable, "notification manifest store unavailable")
	}
	receipt, err := importer.ImportSpacePurgeManifestPage(ctx, request)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "notification manifest import unavailable")
	}
	page := request.GetPage()
	if receipt == nil || receipt.GetProtocolVersion() != 1 || receipt.GetReceiptId() == "" || receipt.GetSpaceId() != request.GetSpaceId() || receipt.GetDeletionOperationId() != request.GetDeletionOperationId() || receipt.GetGeneration() != request.GetScheduleGeneration() || receipt.GetPageIndex() != page.GetPageIndex() || receipt.GetAcceptedCount() != uint64(len(page.GetItemIds())) || !proto.Equal(receipt.GetManifest(), page.GetManifest()) || !bytes.Equal(receipt.GetPageSha256(), page.GetPageSha256()) || receipt.GetManifestSealed() != request.GetSealsManifest() || !notificationReceiptMatches(request, receipt.GetRequestSha256()) || receipt.GetCompletedAt() == nil || receipt.GetCompletedAt().CheckValid() != nil {
		return nil, status.Error(codes.DataLoss, "notification manifest receipt mismatch")
	}
	return &notificationv1.ImportSpacePurgeManifestPageResponse{Receipt: receipt}, nil
}
