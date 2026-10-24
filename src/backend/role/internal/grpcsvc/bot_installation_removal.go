package grpcsvc

import (
	"context"
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/store"
)

func (s *RoleGRPC) ApplyBotInstallationRemoval(ctx context.Context, req *rolev1.ApplyBotInstallationRemovalRequest) (*rolev1.ApplyBotInstallationRemovalResponse, error) {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "role" || p.RPC != rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName || req == nil || p.RequestID == "" || p.RequestID != req.OperationId || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.InstallationID != "" || p.BotScope != "" || p.SpaceID != "" {
		return nil, status.Error(codes.PermissionDenied, "exact verified Bot service principal required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || p.RequestHash != hash {
		return nil, status.Error(codes.PermissionDenied, "exact Bot removal request binding required")
	}
	if err = store.ValidateBotInstallationRemoval(req); err != nil {
		return nil, botInstallationRemovalError(err)
	}
	if s == nil || s.Store == nil {
		return nil, status.Error(codes.Unavailable, "Bot removal persistence unavailable")
	}
	result, err := s.Store.ApplyBotInstallationRemoval(ctx, req)
	if err != nil {
		return nil, botInstallationRemovalError(err)
	}
	return result, nil
}

func botInstallationRemovalError(err error) error {
	switch {
	case errors.Is(err, store.ErrBotRemovalInvalid):
		return status.Error(codes.InvalidArgument, "invalid Bot removal source evidence")
	case errors.Is(err, store.ErrBotRemovalConflict):
		return status.Error(codes.AlreadyExists, "Bot removal operation body conflicts")
	case errors.Is(err, store.ErrBotRemovalDenied):
		return status.Error(codes.PermissionDenied, "Bot removal binding denied")
	case errors.Is(err, store.ErrBotRemovalBlocked):
		return status.Error(codes.FailedPrecondition, "Bot Owner anomaly requires reconciliation")
	default:
		return status.Error(codes.Unavailable, "Bot removal persistence unavailable")
	}
}
