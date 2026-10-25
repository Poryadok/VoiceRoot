package chatcreation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

// Controlled Go protocol component, NOT the Java Auth provider.
func TestAuthFloorSignedProtocolComponent(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, id := uuid.NewString(), uuid.NewString()
	seen := map[string]bool{}
	var tokens []string
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer delegated-not-forwarded", "x-request-id", id))
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-profile-id", "spoof", "authorization", "Bearer outgoing-not-forwarded"))
	checker := &AuthFloorChecker{Issuer: issuer, Client: floorClientFunc(func(call context.Context, req *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error) {
		md, ok := metadata.FromOutgoingContext(call)
		require.True(t, ok)
		require.Len(t, md, 2)
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		tokens = append(tokens, token)
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		p, e := principal.VerifyService(call, token, principal.VerifyConfig{ExpectedIssuer: "chat", ExpectedAudience: "auth", ExpectedRPC: authv1.AuthService_GetChatSessionEpochFloor_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
				if seen[jti] {
					return errors.New("replay")
				}
				seen[jti] = true
				return nil
			}})
		require.NoError(t, e)
		require.Equal(t, "service:chat", p.Subject)
		require.Empty(t, p.AccountID)
		require.Empty(t, p.ProfileID)
		require.Zero(t, p.SessionEpoch)
		deadline, ok := call.Deadline()
		require.True(t, ok)
		require.LessOrEqual(t, time.Until(deadline), 3*time.Second)
		return &authv1.GetChatSessionEpochFloorResponse{SessionEpochFloor: 9}, nil
	})}
	require.NoError(t, checker.CheckSessionEpoch(ctx, account, 9))
	require.NoError(t, checker.CheckSessionEpoch(ctx, account, 9))
	require.NotEqual(t, tokens[0], tokens[1])
	require.Len(t, seen, 2)
	checker.Client = floorClientFunc(func(context.Context, *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error) {
		r := &authv1.GetChatSessionEpochFloorResponse{SessionEpochFloor: 9}
		r.ProtoReflect().SetUnknown([]byte{0xf8, 7, 1})
		return r, nil
	})
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 9), ErrDependencyUnavailable)
	require.ErrorIs(t, checker.CheckSessionEpoch(context.Background(), account, 9), ErrInitiator)
	checker.Issuer = nil
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 9), ErrDependencyUnavailable)
}
