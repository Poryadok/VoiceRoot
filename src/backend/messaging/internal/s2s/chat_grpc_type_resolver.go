package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

// GRPCChatTypeResolver resolves a stored chat type through ChatService. The
// call forwards the authenticated caller metadata so Chat remains the access
// authority for the lookup.
type GRPCChatTypeResolver struct {
	Client chatv1.ChatServiceClient
}

func NewGRPCChatTypeResolver(client chatv1.ChatServiceClient) *GRPCChatTypeResolver {
	return &GRPCChatTypeResolver{Client: client}
}

func (r *GRPCChatTypeResolver) ResolveChatType(ctx context.Context, chatID, _ uuid.UUID) (chatv1.ChatType, error) {
	if r == nil || r.Client == nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, status.Error(codes.Unavailable, "chat service not configured")
	}
	resp, err := r.Client.GetChat(ForwardIncomingMetadata(ctx), &chatv1.GetChatRequest{ChatId: chatID.String()})
	if err != nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, err
	}
	if resp.GetChat() == nil {
		return chatv1.ChatType_CHAT_TYPE_UNSPECIFIED, status.Error(codes.Unavailable, "chat type missing")
	}
	return resp.GetChat().GetType(), nil
}
