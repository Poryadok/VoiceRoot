package principalruntime

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestModerationPrincipalRuntimeRequiresJWKSAndReplayConfiguration(t *testing.T) {
	_, err := New(context.Background(), Config{})
	require.Error(t, err)
	runtime := &Runtime{}
	_, err = runtime.Verify(context.Background(), "token", "/voice.messaging.v1.MessagingService/TombstoneGameMessage", "request", "hash")
	require.Error(t, err)
}
