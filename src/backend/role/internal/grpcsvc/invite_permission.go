package grpcsvc

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
	"voice/backend/role/permissions"
)

func (s *RoleGRPC) checkQualifiedInvites(ctx context.Context, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	if err := principalgrpc.RequireInvitePermission(ctx, req); err != nil {
		return nil, err
	}
	response, err := s.checkQualifiedSpaceDecision(ctx, req, permissions.SpaceManageInvites)
	if err != nil {
		return nil, err
	}
	p, _ := principal.FromContext(ctx)
	if ctx.Err() != nil || !p.ExpiresAt.After(time.Now()) {
		return nil, status.Error(codes.Unavailable, "invite permission expired")
	}
	return response, nil
}
