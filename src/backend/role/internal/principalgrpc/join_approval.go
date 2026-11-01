package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/permissions"
)

// RequireJoinApprovalPermission is one exact global service:space decision.
// It does not authorize other permission names or client-selected resources.
func RequireJoinApprovalPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	return requireSpaceGlobalDecision(ctx, req, permissions.SpaceApproveJoinRequests)
}

func RequireSettingsPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	return requireSpaceGlobalDecision(ctx, req, permissions.SpaceManageSettings)
}

func RequireMatchmakingPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	return requireSpaceGlobalDecision(ctx, req, permissions.SpaceManageMatchmaking)
}

func RequireInvitePermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	return requireSpaceGlobalDecision(ctx, req, permissions.SpaceManageInvites)
}

// RequireBanPermission admits only Space's global ban/unban decision.
func RequireBanPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	return requireSpaceGlobalDecision(ctx, req, permissions.MemberBan)
}

func requireSpaceGlobalDecision(ctx context.Context, req *rolev1.CheckPermissionRequest, permission string) error {
	return requireServiceGlobalDecision(ctx, req, permission, "space")
}

func RequireBotManagementPermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	return requireServiceGlobalDecision(ctx, req, permissions.SpaceManageBots, "bot")
}

func requireServiceGlobalDecision(ctx context.Context, req *rolev1.CheckPermissionRequest, permission, issuer string) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != issuer || p.Subject != "service:"+issuer || p.Audience != "role" || p.RPC != rolev1.RoleService_CheckPermission_FullMethodName || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.InstallationID != "" || p.BotScope != "" || p.SpaceID != "" {
		return status.Error(codes.Unauthenticated, "verified Space approval decision required")
	}
	id, err := uuid.Parse(p.RequestID)
	if err != nil || id == uuid.Nil || id.String() != p.RequestID {
		return status.Error(codes.Unauthenticated, "canonical decision request ID required")
	}
	hash, err := principal.RequestHash(req)
	if req == nil || err != nil || hash != p.RequestHash {
		return status.Error(codes.Unauthenticated, "approval decision binding mismatch")
	}
	if req.PermissionName != permission || req.Chat != nil || req.VoiceRoomId != nil {
		return status.Error(codes.PermissionDenied, "global approval permission required")
	}
	for _, value := range []string{req.SpaceId, req.ProfileId} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return status.Error(codes.InvalidArgument, "canonical approval decision IDs required")
		}
	}
	if len(req.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "unknown approval decision fields")
	}
	return nil
}
