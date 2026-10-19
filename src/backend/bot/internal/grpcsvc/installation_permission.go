package grpcsvc

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

// New install/cancel authority is a closed protected Role decision. The
// ordinary Role client and initiating raw metadata cannot substitute for it.
func (s *BotGRPC) checkInstallationPermission(ctx context.Context, space, profile uuid.UUID) error {
	if s == nil || s.InstallationRole == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.FailedPrecondition, "protected Bot installation permission authority is not configured")
	}
	if space == uuid.Nil || profile == uuid.Nil {
		return status.Error(codes.Unauthenticated, "verified installation actor required")
	}
	request := &rolev1.CheckPermissionRequest{SpaceId: space.String(), ProfileId: profile.String(), PermissionName: "SPACE_MANAGE_BOTS"}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid installation permission request")
	}
	requestID := uuid.NewString()
	token, err := s.PrincipalIssuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: requestID, RequestHash: hash})
	if err != nil {
		return status.Error(codes.Unavailable, "Bot installation permission authority unavailable")
	}
	callCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	callCtx = metadata.NewOutgoingContext(callCtx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
	response, err := s.InstallationRole.CheckPermission(callCtx, request)
	if err != nil || response == nil {
		return status.Error(codes.Unavailable, "Bot installation permission authority unavailable")
	}
	if !response.GetAllowed() {
		return status.Error(codes.PermissionDenied, "SPACE_MANAGE_BOTS required")
	}
	return nil
}
