package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/store"
)

// CheckMessageReadEntitlement is the Chat-owned fetch-time check used by
// history, search, and attachment consumers. The immutable message timestamp
// is evaluated against Chat's membership interval ledger.
func (s *ChatGRPC) CheckMessageReadEntitlement(ctx context.Context, req *chatv1.CheckMessageReadEntitlementRequest) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	if s == nil || s.DM == nil {
		return nil, status.Error(codes.FailedPrecondition, "chat persistence not configured")
	}
	if !authctx.IsInternalService(ctx) || !allowedMessageEntitlementCaller(ctx) {
		return nil, status.Error(codes.PermissionDenied, "messaging, search, or file service identity required")
	}
	chatID, err := parseUUIDField("chat_id", req.GetChatId())
	if err != nil {
		return nil, err
	}
	profileID, err := parseUUIDField("profile_id", req.GetProfileId())
	if err != nil {
		return nil, err
	}
	createdAt := req.GetMessageCreatedAt()
	if createdAt == nil || createdAt.CheckValid() != nil {
		return nil, status.Error(codes.InvalidArgument, "valid message_created_at required")
	}
	dm, ok := s.DM.(*store.DMStore)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition, "managed chat interval store unavailable")
	}
	entitled, err := dm.ManagedChatMessageEntitled(ctx, chatID, profileID, createdAt.AsTime())
	if err != nil {
		if err == store.ErrManagedChatNotFound {
			return nil, status.Error(codes.NotFound, "chat not found")
		}
		return nil, status.Error(codes.Unavailable, "message entitlement unavailable")
	}
	return &chatv1.CheckMessageReadEntitlementResponse{Entitled: entitled}, nil
}

func allowedMessageEntitlementCaller(ctx context.Context) bool {
	for _, service := range []string{"messaging", "search", "file"} {
		if authctx.IsInternalCaller(ctx, service) {
			return true
		}
	}
	return false
}
