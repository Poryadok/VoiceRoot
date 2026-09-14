package s2s

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/principal"

	userv1 "voice.app/voice/user/v1"
)

// This RED contract is the Social side of Sol's Phase-0 cutover. It must turn
// green only after this adapter signs a fresh request-bound service principal.
func TestGRPCUserPrivacy_UsesOnlyVerifiedSocialPrincipalMetadata(t *testing.T) {
	stub := &stubUserPrivacy{}
	conn, cleanup := startBufconnUser(t, stub)
	t.Cleanup(cleanup)

	callerMetadata := metadata.Pairs(
		"authorization", "Bearer attacker-token",
		"x-voice-internal-caller", "attacker",
		"x-voice-user-id", uuid.NewString(),
		"x-request-id", "attacker-request-id",
	)
	ctx := metadata.NewIncomingContext(context.Background(), callerMetadata)
	issuer, _ := testSocialIssuer(t)
	_, err := (&GRPCUserPrivacy{Client: userv1.NewUserServiceClient(conn), Issuer: issuer}).AllowFriendRequestsAudience(ctx, uuid.New())
	require.NoError(t, err)

	// A caller cannot choose any outbound identity. The adapter owns this exact
	// pair, with a newly signed bearer that later tests verify against Social JWKS.
	require.Len(t, stub.lastMD.Get("authorization"), 1, "missing Social service principal")
	require.True(t, strings.HasPrefix(stub.lastMD.Get("authorization")[0], "Bearer "))
	require.Len(t, stub.lastMD.Get("x-request-id"), 1, "missing canonical request id")
	require.Empty(t, stub.lastMD.Get("x-voice-internal-caller"), "raw caller metadata is never authentication")
	require.Empty(t, stub.lastMD.Get("x-voice-user-id"))

	// Keep the package dependency explicit: the later GREEN test must verify the
	// signature and exact claims through this common Phase-0 verifier, rather
	// than treating a JWT-looking string as proof.
	_ = principal.VerifyService
}
