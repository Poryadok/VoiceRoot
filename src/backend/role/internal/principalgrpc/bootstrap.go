package principalgrpc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// RequireOwnerBootstrap authenticates only the existing Space-owned input.
// It does not certify a membership publication or a native contribution.
func RequireOwnerBootstrap(ctx context.Context, req *rolev1.BootstrapSpaceRolesRequest) error {
	p, ok := principal.FromContext(ctx)
	if !ok || p.Kind != "service" || p.Issuer != "space" || p.Subject != "service:space" || p.Audience != "role" || p.RPC != rolev1.RoleService_BootstrapSpaceRoles_FullMethodName || !p.ExpiresAt.After(time.Now()) || p.AccountID != "" || p.ProfileID != "" || p.SessionEpoch != 0 || p.AccountType != "" || p.BotID != "" || p.ActorProfileID != "" || p.SpaceID != "" || p.InstallationID != "" || p.BotScope != "" {
		return status.Error(codes.Unauthenticated, "verified Space bootstrap input required")
	}
	if req == nil || len(req.ProtoReflect().GetUnknown()) != 0 {
		return status.Error(codes.InvalidArgument, "invalid bootstrap input")
	}
	for _, value := range []string{req.SpaceId, req.OwnerProfileId, p.RequestID} {
		id, err := uuid.Parse(value)
		if err != nil || id == uuid.Nil || id.String() != value {
			return status.Error(codes.InvalidArgument, "canonical bootstrap binding required")
		}
	}
	hash, err := principal.RequestHash(req)
	if err != nil || hash != p.RequestHash {
		return status.Error(codes.Unauthenticated, "bootstrap body binding mismatch")
	}
	return nil
}
