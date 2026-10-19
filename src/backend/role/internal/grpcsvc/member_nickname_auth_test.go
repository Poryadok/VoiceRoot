package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
	rolev1 "voice.app/voice/role/v1"
)

func TestNicknamePermission_DirectUnsignedHandlerDeniesBeforeStore(t *testing.T) {
	_, err := (&RoleGRPC{}).CheckPermission(context.Background(), &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "MEMBER_MANAGE_NICKNAMES"})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
