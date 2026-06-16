package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"
)

func (s *MessagingGRPC) UnpinMessagesBySenderInChats(ctx context.Context, req *messagingv1.UnpinMessagesBySenderInChatsRequest) (*messagingv1.UnpinMessagesBySenderInChatsResponse, error) {
	if s == nil || s.Pins == nil {
		return nil, status.Error(codes.FailedPrecondition, "messaging persistence not configured")
	}
	senderID, err := parseUUIDField("sender_profile_id", req.GetSenderProfileId())
	if err != nil {
		return nil, err
	}
	chatIDs := make([]uuid.UUID, 0, len(req.GetChatIds()))
	for _, raw := range req.GetChatIds() {
		id, err := parseUUIDField("chat_id", raw)
		if err != nil {
			return nil, err
		}
		chatIDs = append(chatIDs, id)
	}
	if err := s.Pins.DeletePinsBySenderInChats(ctx, senderID, chatIDs); err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	return &messagingv1.UnpinMessagesBySenderInChatsResponse{}, nil
}
