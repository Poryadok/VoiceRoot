package s2s

import (
	"context"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

// GRPCRolePermissions checks space permissions via RoleService.
type GRPCRolePermissions struct {
	Client rolev1.RoleServiceClient
}

func (g *GRPCRolePermissions) HasSpacePermission(ctx context.Context, spaceID, profileID uuid.UUID, permission string) (bool, error) {
	if g == nil || g.Client == nil {
		return false, status.Error(codes.FailedPrecondition, "role service not configured")
	}
	ctx = ForwardIncomingMetadata(ctx)
	resp, err := g.Client.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permission,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return false, status.Error(codes.Unavailable, "role service unavailable")
		}
		return false, err
	}
	return resp.GetAllowed(), nil
}

// HasChatPermission checks a permission scoped to a text chat in a space.
func (g *GRPCRolePermissions) HasChatPermission(ctx context.Context, spaceID, profileID uuid.UUID, chatID uuid.UUID, permission string) (bool, error) {
	if g == nil || g.Client == nil {
		return false, status.Error(codes.FailedPrecondition, "role service not configured")
	}
	ctx = ForwardIncomingMetadata(ctx)
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	resp, err := g.Client.CheckPermission(ctx, &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permission,
		Chat:           &chatv1.ChatRef{Id: chatID.String(), Type: &group},
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return false, status.Error(codes.Unavailable, "role service unavailable")
		}
		return false, err
	}
	return resp.GetAllowed(), nil
}
