package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	rolev1 "voice.app/voice/role/v1"
)

func TestOrdinaryRoleListenerNicknameDeniedOtherPermissionPreserved(t *testing.T) {
	for _, permission := range []string{"MEMBER_MANAGE_NICKNAMES", "SPACE_MANAGE_INVITES"} {
		called := false
		req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: permission}
		_, err := OwnershipUnaryInterceptor(nil)(context.Background(), req, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		if permission == "MEMBER_MANAGE_NICKNAMES" {
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.False(t, called)
		} else {
			require.NoError(t, err)
			require.True(t, called)
		}
	}
}
