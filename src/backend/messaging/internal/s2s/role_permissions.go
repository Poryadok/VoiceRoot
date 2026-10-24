package s2s

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"voice/backend/pkg/principal"

	chatv1 "voice.app/voice/chat/v1"
	rolev1 "voice.app/voice/role/v1"
)

// GRPCRolePermissions checks space permissions via RoleService.
type GRPCRolePermissions struct {
	Client rolev1.RoleServiceClient
	Issuer *principal.Issuer
}

func (g *GRPCRolePermissions) HasSpacePermission(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return false, status.Error(codes.PermissionDenied, "Messaging Role decisions require chat scope")
}

// HasChatPermission checks a permission scoped to a text chat in a space.
func (g *GRPCRolePermissions) HasChatPermission(ctx context.Context, spaceID, profileID uuid.UUID, chatID uuid.UUID, permission string) (bool, error) {
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	return g.check(ctx, &rolev1.CheckPermissionRequest{
		SpaceId:        spaceID.String(),
		ProfileId:      profileID.String(),
		PermissionName: permission,
		Chat:           &chatv1.ChatRef{Id: chatID.String(), Type: &group},
	})
}

func (g *GRPCRolePermissions) check(ctx context.Context, req *rolev1.CheckPermissionRequest) (bool, error) {
	if g == nil || g.Client == nil || g.Issuer == nil {
		return false, status.Error(codes.Unavailable, "role service principal unavailable")
	}
	hash, err := principal.RequestHash(req)
	if err != nil {
		return false, status.Error(codes.InvalidArgument, "invalid role permission request")
	}
	id := uuid.NewString()
	token, err := g.Issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: rolev1.RoleService_CheckPermission_FullMethodName, RequestID: id, RequestHash: hash})
	if err != nil {
		return false, status.Error(codes.Unavailable, "role service principal unavailable")
	}
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	call = metadata.NewOutgoingContext(call, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	resp, err := g.Client.CheckPermission(call, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return false, status.Error(codes.Unavailable, "role service unavailable")
		}
		return false, err
	}
	return resp.GetAllowed(), nil
}
