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

// RequireNicknamePermission limits the newly protected CheckPermission branch
// to the exact Space-owned global nickname decision. It activates no other name.
func RequireNicknamePermission(ctx context.Context, req *rolev1.CheckPermissionRequest) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "role" || p.RPC != rolev1.RoleService_CheckPermission_FullMethodName || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 {
		return status.Error(codes.Unauthenticated, "verified Space nickname decision required")
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != p.RequestHash {
		return status.Error(codes.Unauthenticated, "nickname decision binding mismatch")
	}
	if req.GetPermissionName() != permissions.MemberManageNicknames || req.Chat != nil || req.VoiceRoomId != nil {
		return status.Error(codes.PermissionDenied, "global nickname permission required")
	}
	for _, value := range []string{req.GetSpaceId(), req.GetProfileId()} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return status.Error(codes.InvalidArgument, "canonical nickname decision IDs required")
		}
	}
	if len(req.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "unknown nickname decision fields")
	}
	return nil
}
