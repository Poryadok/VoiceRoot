package s2s

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCUserPrivacy_DMChecks_NilClientUnavailable(t *testing.T) {
	t.Parallel()
	privacy := &GRPCUserPrivacy{}
	_, err := privacy.AllowDMAudience(context.Background(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
	_, err = privacy.AllowGuestDM(context.Background(), uuid.New())
	require.Equal(t, codes.Unavailable, status.Code(err))
}
