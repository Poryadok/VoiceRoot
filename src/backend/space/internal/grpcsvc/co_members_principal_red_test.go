package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	spacev1 "voice.app/voice/space/v1"
)

// This real handler boundary must deny raw Social identity before touching the
// store. The future protected listener will attach a verified service principal
// and the handler will require it again as defense in depth.
func TestAreCoMembers_RawSocialCallerIsUnauthenticated(t *testing.T) {
	req := &spacev1.AreCoMembersRequest{ProfileIdA: uuid.NewString(), ProfileIdB: uuid.NewString()}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-voice-internal-caller", "social",
		"authorization", "Bearer forged",
	))

	_, err := (&SpaceGRPC{}).AreCoMembers(ctx, req)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
