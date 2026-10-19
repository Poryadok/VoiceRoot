package grpcsvc

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

func (s *BotGRPC) qualifyChatScopes(ctx context.Context, space uuid.UUID, chats []uuid.UUID) (context.Context, error) {
	if len(chats) == 0 {
		return store.WithChatScopeObservations(ctx, nil), nil
	}
	if s.ChatScope == nil || s.PrincipalIssuer == nil {
		return ctx, status.Error(codes.Unavailable, "protected Chat source is not configured")
	}
	unique := make(map[uuid.UUID]bool, len(chats))
	ids := make([]uuid.UUID, 0, len(chats))
	for _, chat := range chats {
		if !unique[chat] {
			unique[chat] = true
			ids = append(ids, chat)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	observations := make([]store.ChatScopeObservation, 0, len(ids))
	for _, chat := range ids {
		request := &chatv1.GetBotChatScopeAuthorityRequest{ProtocolVersion: 1, ChatId: chat.String(), ExpectedSpaceId: space.String(), OperationId: uuid.NewString()}
		hash, err := principal.RequestHash(request)
		if err != nil {
			return ctx, status.Error(codes.InvalidArgument, "invalid Chat scope")
		}
		token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "chat", RPC: chatv1.ChatScopeAuthorityService_GetBotChatScopeAuthority_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
		if err != nil {
			return ctx, status.Error(codes.Unavailable, "Chat source signer unavailable")
		}
		callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
		response, err := s.ChatScope.GetBotChatScopeAuthority(callCtx, request)
		cancel()
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return ctx, status.Error(codes.NotFound, "Chat scope not found")
			}
			return ctx, status.Error(codes.Unavailable, "Chat source unavailable")
		}
		observation, err := store.NewChatScopeObservation(request, response, true)
		if err != nil {
			return ctx, status.Error(codes.Unavailable, "Chat source unavailable")
		}
		observations = append(observations, observation)
	}
	return store.WithChatScopeObservations(ctx, observations), nil
}
