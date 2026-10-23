package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

// The legacy body has no installation or durable operation binding. Bot uses
// the dedicated ApplyBotChatPinRemoval listener instead.
func (s *MessagingGRPC) UnpinMessagesBySenderInChats(context.Context, *messagingv1.UnpinMessagesBySenderInChatsRequest) (*messagingv1.UnpinMessagesBySenderInChatsResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "Bot cleanup requires the protected installation-bound method")
}
