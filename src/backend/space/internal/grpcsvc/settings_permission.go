package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/role/permissions"
)

func (s *SpaceGRPC) requireSettingsPermission(ctx context.Context, spaceID, actor uuid.UUID, permission string) error {
	if permission != permissions.SpaceManageSettings && permission != permissions.SpaceManageMatchmaking {
		return status.Error(codes.PermissionDenied, "unsupported settings permission")
	}
	if s.OwnershipRoles == nil || s.PrincipalIssuer == nil {
		return status.Error(codes.Unavailable, "signed settings permission unavailable")
	}
	req := &rolev1.CheckPermissionRequest{SpaceId: spaceID.String(), ProfileId: actor.String(), PermissionName: permission}
	signed, err := s.ownershipRoleContext(ctx, req, rolev1.RoleService_CheckPermission_FullMethodName, uuid.New())
	if err != nil {
		return err
	}
	response, err := s.OwnershipRoles.CheckPermission(signed, req)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return status.Error(codes.NotFound, "space not found")
		}
		if status.Code(err) == codes.PermissionDenied {
			return status.Error(codes.PermissionDenied, "settings permission denied")
		}
		return status.Error(codes.Unavailable, "settings permission unavailable")
	}
	if !response.GetAllowed() {
		return status.Error(codes.PermissionDenied, "settings permission denied")
	}
	return nil
}
