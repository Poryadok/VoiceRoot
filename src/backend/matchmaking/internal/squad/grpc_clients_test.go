package squad

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/matchmaking/internal/authctx"
)

func TestWithMatchParticipant_PreservesIncomingIdentity(t *testing.T) {
	t.Parallel()
	accepter := uuid.New()
	creator := uuid.New()
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, accepter.String(),
		authctx.HeaderAccountID, uuid.NewString(),
	))

	out, selected, err := withMatchParticipant(incoming, []uuid.UUID{creator, accepter})
	require.NoError(t, err)
	require.Equal(t, accepter, selected)
	md, ok := metadata.FromOutgoingContext(out)
	require.True(t, ok)
	require.Equal(t, []string{accepter.String()}, md.Get(authctx.HeaderProfileID))
	require.Equal(t, []string{"matchmaking"}, md.Get("x-voice-internal-caller"))
	require.Len(t, md.Get(authctx.HeaderAccountID), 1)
}
