package principalgrpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// RequireBotGlobalTerminal admits only the saved minimal Bot/root assertion.
// It conveys no Space cleanup, human mutation or erased-source authority.
func RequireBotGlobalTerminal(ctx context.Context, req *rolev1.RegisterBotGlobalTerminalRequest) error {
	if req == nil || req.ProtocolVersion != 1 || unknownRoleRequest(req.ProtoReflect(), 0) || !publicRoleUUID(req.BotId) || !publicRoleUUID(req.DecisionId) {
		return status.Error(codes.InvalidArgument, "invalid Bot terminal request")
	}
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "role" || p.RPC != rolev1.RoleService_RegisterBotGlobalTerminal_FullMethodName || p.RequestID != req.DecisionId || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.InstallationID != "" || p.BotScope != "" || p.SpaceID != "" {
		return status.Error(codes.PermissionDenied, "exact Bot terminal service binding required")
	}
	return nil
}
