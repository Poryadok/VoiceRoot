package spaceprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/responseprincipal"
	"voice/backend/pkg/principal"
)

type responseVerifier struct {
	v      responseprincipal.Verifier
	mutate bool
}

func (v responseVerifier) Verify(context.Context, string, string, string, string) (principal.Principal, error) {
	return principal.Principal{}, nil
}
func (v responseVerifier) VerifyResponse(ctx context.Context, req *matchmakingv1.RespondToMatchRequest) (context.Context, error) {
	verified, err := v.v.Verify(ctx, req)
	if v.mutate {
		req.Accept = !req.Accept
	}
	return verified, err
}

func TestProtectedResponseUsesExactPrivateSubjectForBothDecisions(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	for _, accept := range []bool{false, true} {
		for _, mutate := range []bool{false, true} {
			req := &matchmakingv1.RespondToMatchRequest{MatchId: uuid.NewString(), Accept: accept, OperationId: uuid.NewString()}
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			account, profile := uuid.NewString(), uuid.NewString()
			token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName, RequestID: req.OperationId, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)})
			require.NoError(t, err)
			v := responseVerifier{mutate: mutate, v: responseprincipal.Verifier{Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", req.OperationId))
			called := false
			_, err = ProtectedUnaryInterceptor(v)(ctx, req, &grpc.UnaryServerInfo{FullMethod: matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
				called = true
				s, ok := responseprincipal.SubjectFromContext(ctx, req)
				require.True(t, ok)
				require.Equal(t, profile, s.ProfileID.String())
				require.Equal(t, account, s.AccountID.String())
				return &matchmakingv1.RespondToMatchResponse{}, nil
			})
			if mutate {
				require.Error(t, err)
				require.False(t, called)
			} else {
				require.NoError(t, err)
				require.True(t, called)
			}
		}
	}
}

func TestNativeResponseListenerDeniesOrdinaryAndMissingVerifier(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName}
	for _, accept := range []bool{false, true} {
		req := &matchmakingv1.RespondToMatchRequest{MatchId: uuid.NewString(), OperationId: uuid.NewString(), Accept: accept}
		for _, interceptor := range []grpc.UnaryServerInterceptor{NativeOrdinaryUnaryInterceptor(), ProtectedUnaryInterceptor(nil)} {
			called := false
			_, err := interceptor(context.Background(), req, info, func(context.Context, any) (any, error) { called = true; return nil, nil })
			require.Error(t, err)
			require.False(t, called)
		}
		called := false
		_, err := OrdinaryUnaryInterceptor()(context.Background(), req, info, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.NoError(t, err)
		require.True(t, called, "explicit pre-cut ordinary composition is preserved")
	}
}
