package grpcsvc

import (
	"context"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/internal/store"
)

func (s *RoleGRPC) RegisterBotGlobalTerminal(ctx context.Context, req *rolev1.RegisterBotGlobalTerminalRequest) (*rolev1.RegisterBotGlobalTerminalResponse, error) {
	if err := principalgrpc.RequireBotGlobalTerminal(ctx, req); err != nil {
		return nil, err
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Role terminal persistence unavailable")
	}
	result, err := s.Store.RegisterBotGlobalTerminal(ctx, req)
	switch {
	case err == nil:
		return result, nil
	case errors.Is(err, store.ErrBotGlobalTerminalInvalid):
		return nil, status.Error(codes.InvalidArgument, "invalid Role terminal request")
	case errors.Is(err, store.ErrBotGlobalTerminalDenied):
		return nil, status.Error(codes.PermissionDenied, "exact Role terminal authority required")
	case errors.Is(err, store.ErrBotGlobalTerminalConflict):
		return nil, status.Error(codes.FailedPrecondition, "Role terminal Bot/root binding conflicts")
	default:
		return nil, status.Error(codes.Unavailable, "Role terminal dependency unavailable")
	}
}
