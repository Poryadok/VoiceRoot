package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUserGRPCPrivacy_AllowDM_NilClientUnavailable(t *testing.T) {
	t.Parallel()
	_, err := (&UserGRPCPrivacy{}).AllowDMAudience(context.Background(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestSocialGRPCBlocks_NilClientUnavailable(t *testing.T) {
	t.Parallel()
	var blocks *SocialGRPCBlocks
	_, err := blocks.AccountPairBlocked(context.Background(), uuid.New(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
}
