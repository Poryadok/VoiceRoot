package grpcsvc

import (
	"context"
	"math"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type SpaceReportAttachmentStore interface {
	ReadSpaceReportAttachment(context.Context, uuid.UUID, uuid.UUID, uint64) (*chatv1.ReadSpaceReportAttachmentResponse, error)
}

func (s *SpaceChatScopeAuthorityGRPC) ReadSpaceReportAttachment(ctx context.Context, req *chatv1.ReadSpaceReportAttachmentRequest) (*chatv1.ReadSpaceReportAttachmentResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Chat attachment principal required")
	}
	if p.Kind != "service" || p.Issuer != "messaging" || p.Subject != "service:messaging" || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return nil, status.Error(codes.PermissionDenied, "Messaging service principal required")
	}
	parse := func(raw string) (uuid.UUID, bool) {
		id, err := uuid.Parse(raw)
		return id, err == nil && id != uuid.Nil && id.String() == raw
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.ProtocolVersion != 1 || req.SavedAttachmentGeneration == 0 || req.SavedAttachmentGeneration > math.MaxInt64 {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat attachment request")
	}
	chat, chatOK := parse(req.ChatId)
	space, spaceOK := parse(req.ExpectedSpaceId)
	_, operationOK := parse(req.OperationId)
	if !chatOK || !spaceOK || !operationOK {
		return nil, status.Error(codes.InvalidArgument, "invalid Chat attachment request")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || p.Audience != "chat" || p.RPC != chatv1.ChatScopeAuthorityService_ReadSpaceReportAttachment_FullMethodName || p.RequestID != req.OperationId || p.RequestHash != hash {
		return nil, status.Error(codes.Unauthenticated, "invalid Chat attachment principal binding")
	}
	if s == nil {
		return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
	}
	store, ok := s.Store.(SpaceReportAttachmentStore)
	if !ok || store == nil {
		return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
	}
	response, err := store.ReadSpaceReportAttachment(ctx, chat, space, req.SavedAttachmentGeneration)
	if err != nil {
		if ctx.Err() != nil {
			return nil, status.FromContextError(ctx.Err()).Err()
		}
		return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
	}
	if response == nil || response.ProtocolVersion != 1 || response.ChatId != req.ChatId || response.ExpectedSpaceId != req.ExpectedSpaceId || response.SavedAttachmentGeneration != req.SavedAttachmentGeneration || len(response.ProtoReflect().GetUnknown()) != 0 {
		return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
	}
	switch response.Disposition {
	case chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNKNOWN, chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_UNCHANGED:
		if response.Reason != 0 || response.OwnerEventId != "" || response.OwnerRevision != 0 {
			return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
		}
	case chatv1.SpaceReportAttachmentDisposition_SPACE_REPORT_ATTACHMENT_DISPOSITION_RETIRED:
		_, eventOK := parse(response.OwnerEventId)
		if !eventOK || response.OwnerRevision == 0 || (response.Reason != chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_DELETED && response.Reason != chatv1.SpaceReportAttachmentRetirementReason_SPACE_REPORT_ATTACHMENT_RETIREMENT_REASON_TARGET_MOVED) {
			return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
		}
	default:
		return nil, status.Error(codes.Unavailable, "Chat attachment authority unavailable")
	}
	response.OperationId = req.OperationId
	return response, nil
}
