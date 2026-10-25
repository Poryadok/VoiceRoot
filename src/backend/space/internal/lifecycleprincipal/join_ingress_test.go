package lifecycleprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestJoinIngressRequiresProtectedExactFiniteCall(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	for _, tc := range []struct {
		name, method string
		request      proto.Message
		requireCall  func(context.Context, proto.Message) (*VerifiedAdmissionCall, error)
	}{
		{"direct", spacev1.SpaceService_JoinSpace_FullMethodName, &spacev1.JoinSpaceRequest{SpaceId: uuid.NewString()}, RequireAdmissionJoinSpace},
		{"invite", spacev1.SpaceService_JoinByInvite_FullMethodName, &spacev1.JoinByInviteRequest{Code: "exact-invite"}, RequireAdmissionJoinInvite},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now().UTC().Truncate(time.Second)
			issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "join", PrivateKey: key, Clock: func() time.Time { return now }})
			require.NoError(t, err)
			hash, err := principal.RequestHash(tc.request)
			require.NoError(t, err)
			id := uuid.NewString()
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "space", RPC: tc.method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, AccountType: "regular", ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
			called := false
			_, err = OrdinaryUnary()(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Equal(t, codes.PermissionDenied, status.Code(err))
			require.False(t, called)
			verifier := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }, Clock: func() time.Time { return now }}
			_, err = verifier.Unary()(ctx, tc.request, &grpc.UnaryServerInfo{FullMethod: tc.method}, func(ctx context.Context, _ any) (any, error) {
				called = true
				call, err := tc.requireCall(ctx, tc.request)
				require.NoError(t, err)
				_, err = call.ActorFor(tc.request)
				require.NoError(t, err)
				now = now.Add(5 * time.Second)
				_, err = call.ActorFor(tc.request)
				require.Equal(t, codes.Unavailable, status.Code(err))
				return nil, nil
			})
			require.NoError(t, err)
			require.True(t, called)
		})
	}
}
