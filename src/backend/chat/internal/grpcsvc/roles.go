package grpcsvc

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"
)

func requireSpacePermission(
	ctx context.Context,
	roles rolev1.RoleServiceClient,
	spaceID, profileID uuid.UUID,
	permission string,
) error {
	if roles == nil {
		return status.Error(codes.FailedPrecondition, "role service not configured")
	}
	resp, err := roles.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permission,
	})
	if err != nil {
		if status.Code(err) == codes.Unavailable {
			return status.Error(codes.Unavailable, "role service unavailable")
		}
		return status.Error(codes.Internal, err.Error())
	}
	if !resp.GetAllowed() {
		return status.Error(codes.PermissionDenied, "permission denied")
	}
	return nil
}

func canSetSpaceChatSlowMode(
	ctx context.Context,
	roles rolev1.RoleServiceClient,
	spaceID, caller uuid.UUID,
) error {
	err := requireSpacePermission(ctx, roles, spaceID, caller, "TEXT_CHAT_SET_SLOW_MODE")
	if err == nil {
		return nil
	}
	if status.Code(err) != codes.PermissionDenied {
		return err
	}
	return requireSpacePermission(ctx, roles, spaceID, caller, "TEXT_CHAT_MANAGE_SETTINGS")
}
