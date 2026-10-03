package grpcsvc

import (
	"context"
	"errors"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	notificationv1 "voice.app/voice/notification/v1"
	"voice/backend/pkg/principal"
)

// LifecycleManifestOwners signs the exact protobuf request for each owner and
// sends only the signed Space service identity. The supplied connections must
// use the owners' protected mTLS listeners.
type LifecycleManifestOwners struct {
	Issuer       *principal.Issuer
	Chat         chatv1.ChatServiceClient
	Messaging    messagingv1.MessagingServiceClient
	File         filev1.FileServiceClient
	Notification notificationv1.NotificationServiceClient
}

func (o *LifecycleManifestOwners) ImportNotificationManifestPage(ctx context.Context, req *notificationv1.ImportSpacePurgeManifestPageRequest) (*notificationv1.ImportSpacePurgeManifestPageResponse, error) {
	if o == nil || o.Notification == nil {
		return nil, status.Error(codes.Unavailable, "Notification lifecycle transport unavailable")
	}
	signed, err := o.signed(ctx, "notification", notificationv1.NotificationService_ImportSpacePurgeManifestPage_FullMethodName, req, req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	return o.Notification.ImportSpacePurgeManifestPage(signed, req)
}

func (o *LifecycleManifestOwners) PrepareChatManifest(ctx context.Context, req *chatv1.PrepareSpaceDeletionManifestRequest) (*chatv1.PrepareSpaceDeletionManifestResponse, error) {
	if o == nil || o.Chat == nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle transport unavailable")
	}
	signed, err := o.signed(ctx, "chat", chatv1.ChatService_PrepareSpaceDeletionManifest_FullMethodName, req, req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	return o.Chat.PrepareSpaceDeletionManifest(signed, req)
}

func (o *LifecycleManifestOwners) GetChatManifestPage(ctx context.Context, req *chatv1.GetSpacePurgeManifestPageRequest) (*chatv1.GetSpacePurgeManifestPageResponse, error) {
	if o == nil || o.Chat == nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle transport unavailable")
	}
	signed, err := o.signed(ctx, "chat", chatv1.ChatService_GetSpacePurgeManifestPage_FullMethodName, req, req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	return o.Chat.GetSpacePurgeManifestPage(signed, req)
}

func (o *LifecycleManifestOwners) ImportMessagingManifestPage(ctx context.Context, req *messagingv1.ImportSpacePurgeManifestPageRequest) (*messagingv1.ImportSpacePurgeManifestPageResponse, error) {
	if o == nil || o.Messaging == nil {
		return nil, status.Error(codes.Unavailable, "Messaging lifecycle transport unavailable")
	}
	signed, err := o.signed(ctx, "messaging", messagingv1.MessagingService_ImportSpacePurgeManifestPage_FullMethodName, req, req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	return o.Messaging.ImportSpacePurgeManifestPage(signed, req)
}

func (o *LifecycleManifestOwners) PrepareFileManifest(ctx context.Context, req *filev1.PrepareSpaceDeletionReferenceManifestRequest) (*filev1.PrepareSpaceDeletionReferenceManifestResponse, error) {
	if o == nil || o.File == nil {
		return nil, status.Error(codes.Unavailable, "File lifecycle transport unavailable")
	}
	signed, err := o.signed(ctx, "file", filev1.FileService_PrepareSpaceDeletionReferenceManifest_FullMethodName, req, req.GetDeletionOperationId())
	if err != nil {
		return nil, err
	}
	return o.File.PrepareSpaceDeletionReferenceManifest(signed, req)
}

func (o *LifecycleManifestOwners) signed(ctx context.Context, audience, method string, request proto.Message, requestID string) (context.Context, error) {
	if o == nil || o.Issuer == nil || request == nil || strings.TrimSpace(requestID) == "" {
		return nil, status.Error(codes.Unavailable, "Space lifecycle principal runtime unavailable")
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle owner request")
	}
	token, err := o.Issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Space lifecycle principal signing unavailable")
	}
	if ctx == nil {
		return nil, errors.New("space lifecycle owner context is nil")
	}
	return metadata.NewOutgoingContext(ctx, metadata.Pairs(
		"authorization", "Bearer "+token,
		"x-request-id", requestID,
	)), nil
}
