package grpcsvc

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type realtimeLifecycleStore interface {
	ResolveChatLifecycle(context.Context, uuid.UUID) (*chatv1.ResolveChatLifecycleResponse, error)
}
type RealtimeLifecycleGRPC struct {
	chatv1.UnimplementedChatRealtimeLifecycleServiceServer
	Store realtimeLifecycleStore
}

func (s *RealtimeLifecycleGRPC) ResolveChatLifecycle(ctx context.Context, req *chatv1.ResolveChatLifecycleRequest) (*chatv1.ResolveChatLifecycleResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle enrollment request")
	}
	method := chatv1.ChatRealtimeLifecycleService_ResolveChatLifecycle_FullMethodName
	verified, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || verified.Kind != "service" || verified.Issuer != "realtime" || verified.Subject != "service:realtime" || verified.Audience != "chat" || verified.RPC != method || verified.RequestID != req.GetOperationId() || verified.RequestHash != hash || verified.AccountID != "" || verified.ProfileID != "" || verified.SessionEpoch != 0 {
		return nil, status.Error(codes.Unauthenticated, "request-bound Realtime principal required")
	}
	chat, parseErr := uuid.Parse(req.GetChatId())
	operation, opErr := uuid.Parse(req.GetOperationId())
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || req.GetProtocolVersion() != 1 || parseErr != nil || chat == uuid.Nil || chat.String() != req.GetChatId() || opErr != nil || operation == uuid.Nil || operation.String() != req.GetOperationId() {
		return nil, status.Error(codes.InvalidArgument, "invalid lifecycle enrollment request")
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle enrollment unavailable")
	}
	result, err := s.Store.ResolveChatLifecycle(ctx, chat)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "chat unavailable")
	}
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Chat lifecycle enrollment unavailable")
	}
	return result, nil
}
