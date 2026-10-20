package grpcsvc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

type BotPurgeInstallationStore interface {
	ReconcileBotPurgeInstallation(context.Context, *botmembershipprincipal.VerifiedPurgeInstallation, *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.BotPurgeInstallationReceipt, error)
}

func (s *SpaceGRPC) ReconcileBotPurgeInstallation(context.Context, *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.ReconcileBotPurgeInstallationResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
}

func (s *BotMembershipGRPC) ReconcileBotPurgeInstallation(ctx context.Context, request *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.ReconcileBotPurgeInstallationResponse, error) {
	proof, err := botmembershipprincipal.RequirePurgeInstallation(ctx, request)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "native Bot P3 child unavailable")
	}
	st, ok := s.Store.(BotPurgeInstallationStore)
	if !ok {
		return nil, status.Error(codes.Unavailable, "native Bot P3 child unavailable")
	}
	receipt, err := st.ReconcileBotPurgeInstallation(ctx, proof, request)
	if err != nil {
		return nil, mapBotMembershipError(err)
	}
	if receipt == nil {
		return nil, status.Error(codes.Unavailable, "native Bot P3 child unavailable")
	}
	if _, err = proof.Check(request); err != nil {
		return nil, err
	}
	return &spacev1.ReconcileBotPurgeInstallationResponse{Receipt: receipt}, nil
}
