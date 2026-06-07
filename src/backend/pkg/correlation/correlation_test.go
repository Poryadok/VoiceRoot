package correlation

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestFromGRPC_Empty(t *testing.T) {
	require.Equal(t, "", FromGRPC(context.Background()))
	require.Equal(t, "", FromGRPC(nil)) //nolint:staticcheck // explicit nil guard
}

func TestFromGRPC_RoundTrip(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(GRPCMetadataKey, "abc-123"))
	require.Equal(t, "abc-123", FromGRPC(ctx))
}

func TestWithGRPC_Outgoing(t *testing.T) {
	ctx := WithGRPC(context.Background(), "out-1")
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"out-1"}, md.Get(GRPCMetadataKey))
}

func TestGenerateRequestID_NonEmpty(t *testing.T) {
	id := GenerateRequestID()
	require.NotEmpty(t, id)
	require.NotEqual(t, "request-id-unavailable", id)
	require.Len(t, id, 32)
}
