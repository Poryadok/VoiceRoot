package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
)

type BotInstallationScopeReader interface {
	CheckCurrentInstallationScope(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, uuid.UUID, string) (bool, error)
}

func (s *BotGRPC) CheckBotInstallationScope(ctx context.Context, req *botv1.CheckBotInstallationScopeRequest) (*botv1.CheckBotInstallationScopeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "installation binding required")
	}
	if err := requireBotLifecyclePrincipal(ctx, botv1.BotService_CheckBotInstallationScope_FullMethodName, req); err != nil {
		return nil, err
	}
	var ids [4]uuid.UUID
	for i, value := range []string{req.BotId, req.SpaceId, req.ActorProfileId, req.InstallationId} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return nil, status.Error(codes.InvalidArgument, "canonical nonzero installation IDs required")
		}
		ids[i] = id
	}
	if req.BotScope != "TEXT_CHAT_CREATE_IN_SPACE" {
		return nil, status.Error(codes.InvalidArgument, "unsupported Space installation scope")
	}
	if s == nil || s.InstallationScopeReader == nil {
		return nil, status.Error(codes.Unavailable, "Bot installation authority unavailable")
	}
	allowed, err := s.InstallationScopeReader.CheckCurrentInstallationScope(ctx, ids[0], ids[1], ids[2], ids[3], req.BotScope)
	if err != nil {
		return nil, status.Error(codes.Unavailable, "Bot installation authority unavailable")
	}
	return &botv1.CheckBotInstallationScopeResponse{Allowed: allowed, BotId: req.BotId, SpaceId: req.SpaceId, ActorProfileId: req.ActorProfileId, InstallationId: req.InstallationId, BotScope: req.BotScope}, nil
}
