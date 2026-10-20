package grpcsvc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/space/internal/botmembershipprincipal"
)

type BotGlobalTerminalStore interface {
	RegisterBotGlobalTerminal(context.Context, *botmembershipprincipal.VerifiedGlobalTerminal, *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.BotGlobalTerminalReceipt, error)
}

func (s *SpaceGRPC) RegisterBotGlobalTerminal(context.Context, *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
	return nil, status.Error(codes.PermissionDenied, "protected Bot membership listener required")
}
func (s *BotMembershipGRPC) RegisterBotGlobalTerminal(ctx context.Context, r *spacev1.RegisterBotGlobalTerminalRequest) (*spacev1.RegisterBotGlobalTerminalResponse, error) {
	proof, err := botmembershipprincipal.RequireGlobalTerminal(ctx, r)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Bot terminal registration unavailable")
	}
	st, ok := s.Store.(BotGlobalTerminalStore)
	if !ok {
		return nil, status.Error(codes.Unavailable, "Bot terminal registration unavailable")
	}
	receipt, err := st.RegisterBotGlobalTerminal(ctx, proof, r)
	if err != nil {
		return nil, mapBotMembershipError(err)
	}
	if receipt == nil {
		return nil, status.Error(codes.Unavailable, "Bot terminal registration unavailable")
	}
	if _, err = proof.Check(r); err != nil {
		return nil, err
	}
	return &spacev1.RegisterBotGlobalTerminalResponse{Receipt: receipt}, nil
}
