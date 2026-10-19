package principalgrpc

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// Only the Bot owner service may assert its persisted cleanup decision. The
// handler independently validates the complete native/legacy/orphan evidence.
func RequireBotRemoval(ctx context.Context, req *rolev1.ApplyBotInstallationRemovalRequest) error {
	if req == nil || unknownRoleRequest(req.ProtoReflect(), 0) || !publicRoleUUID(req.GetOperationId()) {
		return status.Error(codes.InvalidArgument, "invalid Bot removal request")
	}
	p, ok := principal.FromContext(ctx)
	hash, err := principal.RequestHash(req)
	if !ok || err != nil || p.Kind != "service" || p.Issuer != "bot" || p.Subject != "service:bot" || p.Audience != "role" || p.RPC != rolev1.RoleService_ApplyBotInstallationRemoval_FullMethodName || p.RequestID != req.OperationId || p.RequestHash != hash || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.InstallationID != "" || p.BotScope != "" || p.SpaceID != "" {
		return status.Error(codes.PermissionDenied, "exact Bot cleanup service binding required")
	}
	return nil
}
