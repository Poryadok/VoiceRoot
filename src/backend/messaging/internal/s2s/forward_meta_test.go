package s2s

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestForwardIncomingMetadata(t *testing.T) {
	t.Parallel()
	require.Equal(t, context.Background(), ForwardIncomingMetadata(context.Background()))

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-user-id", "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"))
	out := ForwardIncomingMetadata(ctx)
	md, ok := metadata.FromOutgoingContext(out)
	require.True(t, ok)
	require.Equal(t, []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"}, md.Get("x-voice-user-id"))
}
