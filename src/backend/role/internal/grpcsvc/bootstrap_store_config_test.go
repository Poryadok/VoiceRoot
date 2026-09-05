package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	rolev1 "voice.app/voice/role/v1"

	"voice/backend/role/internal/store"
)

func TestBootstrapSpaceRoles_UnconfiguredStoreReturnsInternal(t *testing.T) {
	service := &RoleGRPC{Store: &store.RoleStore{}}
	_, err := service.BootstrapSpaceRoles(context.Background(), &rolev1.BootstrapSpaceRolesRequest{
		SpaceId:        uuid.New().String(),
		OwnerProfileId: uuid.New().String(),
	})
	require.Equal(t, codes.Internal, status.Code(err))
}
