package s2s_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/notification/internal/s2s"
)

func TestContext_AttachesInternalCaller(t *testing.T) {
	ctx := s2s.Context(context.Background())
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(t, ok)
	require.Equal(t, []string{"notification"}, md.Get("x-voice-internal-caller"))
}
