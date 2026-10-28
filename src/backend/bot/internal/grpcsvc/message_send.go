package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func (s *BotGRPC) postSpaceBotMessage(ctx context.Context, bot *store.BotRow, message *messagingv1.SendMessageRequest) (*messagingv1.Message, error) {
	if s == nil || s.Store == nil || s.ProtectedMessaging == nil || s.PrincipalIssuer == nil {
		return nil, status.Error(codes.Unavailable, "protected Bot message sender unavailable")
	}
	if bot == nil || message == nil {
		return nil, status.Error(codes.InvalidArgument, "Bot message required")
	}
	chat, err := parseUUID("chat.id", message.GetChat().GetId())
	if err != nil {
		return nil, err
	}
	var result *messagingv1.Message
	err = s.Store.WithMessageSend(ctx, bot.Authority(), bot.ActorProfileID, chat, func(call context.Context, binding *commonv1.BotMessageSendBinding) error {
		req := &messagingv1.SendBotMessageRequest{Message: proto.Clone(message).(*messagingv1.SendMessageRequest), Binding: binding}
		hash, err := principal.RequestHash(req)
		if err != nil {
			return err
		}
		id := uuid.NewString()
		token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "messaging", RPC: messagingv1.MessagingService_SendBotMessage_FullMethodName, RequestID: id, RequestHash: hash})
		if err != nil {
			return err
		}
		// Replace all caller metadata; neither the human nor raw Bot actor is authority.
		call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
		response, err := s.ProtectedMessaging.SendBotMessage(call, req)
		if err != nil {
			return err
		}
		if response == nil || response.Message == nil {
			return status.Error(codes.Unavailable, "Bot message response missing")
		}
		result = response.Message
		return nil
	})
	if errors.Is(err, store.ErrMessageSendDenied) {
		return nil, status.Error(codes.PermissionDenied, "Bot message send authority denied")
	}
	if err != nil {
		if status.Code(err) != codes.Unknown {
			return nil, err
		}
		return nil, status.Error(codes.Unavailable, "Bot message send unavailable")
	}
	return result, nil
}
