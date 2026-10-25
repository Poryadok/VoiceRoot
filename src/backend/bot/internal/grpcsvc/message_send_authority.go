package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"time"
	botv1 "voice.app/voice/bot/v1"
	commonv1 "voice.app/voice/common/v1"
	"voice/backend/bot/internal/store"
	"voice/backend/pkg/principal"
)

type BotMessageAuthorityReader interface {
	CheckCurrentMessageSendAuthority(context.Context, *commonv1.BotMessageSendBinding) (bool, error)
}

func (s *BotGRPC) CheckBotMessageSendAuthority(ctx context.Context, req *botv1.CheckBotMessageSendAuthorityRequest) (*botv1.CheckBotMessageSendAuthorityResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "verified Messaging principal required")
	}
	if p.Kind != "service" || p.Issuer != "messaging" || p.Subject != "service:messaging" {
		return nil, status.Error(codes.PermissionDenied, "Messaging principal required")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 || store.ValidateMessageSendBinding(req.GetBinding()) != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid message send binding")
	}
	hash, err := principal.RequestHash(req)
	id, idErr := uuid.Parse(p.RequestID)
	if err != nil || idErr != nil || id == uuid.Nil || id.String() != p.RequestID || p.Audience != "bot" || p.RPC != botv1.BotService_CheckBotMessageSendAuthority_FullMethodName || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return nil, status.Error(codes.Unauthenticated, "invalid Messaging principal binding")
	}
	if s == nil || s.MessageAuthorityReader == nil {
		return nil, status.Error(codes.Unavailable, "Bot send authority unavailable")
	}
	if !p.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unauthenticated, "expired Messaging principal")
	}
	ctx, cancel := context.WithDeadline(ctx, p.ExpiresAt)
	defer cancel()
	ctx, bounded := context.WithTimeout(ctx, 5*time.Second)
	defer bounded()
	allowed, err := s.MessageAuthorityReader.CheckCurrentMessageSendAuthority(ctx, req.Binding)
	if err != nil || ctx.Err() != nil || !p.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unavailable, "Bot send authority unavailable")
	}
	return &botv1.CheckBotMessageSendAuthorityResponse{Binding: proto.Clone(req.Binding).(*commonv1.BotMessageSendBinding), Allowed: allowed}, nil
}
