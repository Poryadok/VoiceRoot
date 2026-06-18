package s2s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/story/internal/s2s"
)

func TestForwardIncomingMetadata_copiesToOutgoing(t *testing.T) {
	in := metadata.Pairs("x-voice-profile-id", "00000000-0000-0000-0000-000000000001")
	ctx := metadata.NewIncomingContext(context.Background(), in)
	out := s2s.ForwardIncomingMetadata(ctx)
	md, ok := metadata.FromOutgoingContext(out)
	require.True(t, ok)
	require.Equal(t, []string{"00000000-0000-0000-0000-000000000001"}, md.Get("x-voice-profile-id"))
}

func TestForwardIncomingMetadata_noIncomingUnchanged(t *testing.T) {
	ctx := context.Background()
	out := s2s.ForwardIncomingMetadata(ctx)
	_, ok := metadata.FromOutgoingContext(out)
	require.False(t, ok)
}
