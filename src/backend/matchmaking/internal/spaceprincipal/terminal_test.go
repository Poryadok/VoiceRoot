package spaceprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/terminalprincipal"
	"voice/backend/pkg/principal"
)

type terminalTransportVerifier struct {
	verify func(context.Context, proto.Message, string) (context.Context, error)
}

func (v terminalTransportVerifier) Verify(context.Context, string, string, string, string) (principal.Principal, error) {
	return principal.Principal{}, errors.New("not service")
}
func (v terminalTransportVerifier) VerifyTerminal(ctx context.Context, p proto.Message, m string) (context.Context, error) {
	return v.verify(ctx, p, m)
}

func TestProtectedTerminalTransportRequiresPrivateBoundActor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	req := &matchmakingv1.GetSearchStatusRequest{SessionId: uuid.NewString()}
	method := matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "guest", ClientExpiresAt: time.Now().Add(time.Minute)})
	require.NoError(t, err)
	incoming := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	verifier := terminalprincipal.Verifier{KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}
	transport := terminalTransportVerifier{verify: verifier.Verify}
	called := false
	handler := func(ctx context.Context, _ any) (any, error) {
		_, ok := terminalprincipal.SubjectFromContext(ctx, req, method)
		require.True(t, ok)
		called = true
		return "ok", nil
	}
	_, err = ProtectedUnaryInterceptor(transport)(incoming, req, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.NoError(t, err)
	require.True(t, called)
	called = false
	transport.verify = func(ctx context.Context, p proto.Message, m string) (context.Context, error) {
		verified, e := verifier.Verify(ctx, p, m)
		req.SessionId = uuid.NewString()
		return verified, e
	}
	_, err = ProtectedUnaryInterceptor(transport)(incoming, req, &grpc.UnaryServerInfo{FullMethod: method}, handler)
	require.Error(t, err)
	require.False(t, called)
}
func TestNativeOrdinaryListenerExposesOnlyPublicCatalogue(t *testing.T) {
	allowed := map[string]bool{"GetGame": true, "ListGames": true, "SearchGames": true}
	for _, method := range matchmakingv1.MatchmakingService_ServiceDesc.Methods {
		t.Run(method.MethodName, func(t *testing.T) {
			called := false
			_, err := NativeOrdinaryUnaryInterceptor()(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/voice.matchmaking.v1.MatchmakingService/" + method.MethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
			if allowed[method.MethodName] {
				require.NoError(t, err)
				require.True(t, called)
			} else {
				require.Error(t, err)
				require.False(t, called)
			}
		})
	}
}
