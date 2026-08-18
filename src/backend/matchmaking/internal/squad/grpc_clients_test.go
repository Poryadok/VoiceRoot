package squad

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/matchmaking/internal/authctx"
)

func TestWithCreatorProfile_ReplacesIncomingProfile(t *testing.T) {
	t.Parallel()
	accepter := uuid.New()
	creator := uuid.New()
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, accepter.String(),
		authctx.HeaderAccountID, uuid.NewString(),
	))

	out := withCreatorProfile(incoming, creator)
	md, ok := metadata.FromOutgoingContext(out)
	require.True(t, ok)
	require.Equal(t, []string{creator.String()}, md.Get(authctx.HeaderProfileID))
	require.Equal(t, []string{"matchmaking"}, md.Get("x-voice-internal-caller"))
	require.Len(t, md.Get(authctx.HeaderAccountID), 1)
}
