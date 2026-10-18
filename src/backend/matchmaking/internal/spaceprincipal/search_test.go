package spaceprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/matchmaking/internal/searchprincipal"
	"voice/backend/pkg/principal"
)

type searchVerifier struct {
	t      *testing.T
	v      searchprincipal.Verifier
	mutate bool
}

func (s searchVerifier) Verify(context.Context, string, string, string, string) (principal.Principal, error) {
	s.t.Fatal("StartSearch entered Space service verifier")
	return principal.Principal{}, nil
}
func (s searchVerifier) VerifySearch(ctx context.Context, r *matchmakingv1.StartSearchRequest) (context.Context, error) {
	out, err := s.v.Verify(ctx, r)
	if s.mutate {
		r.Mode = "mutated-after-verification"
	}
	return out, err
}
func TestProtectedStartSearchUsesDedicatedVerifiedSubjectAndRejectsMutation(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	for _, mutate := range []bool{false, true} {
		req := &matchmakingv1.StartSearchRequest{GameId: uuid.NewString(), Mode: "duo"}
		op := uuid.NewString()
		account, profile := uuid.NewString(), uuid.NewString()
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "matchmaking", RPC: matchmakingv1.MatchmakingService_StartSearch_FullMethodName, RequestID: op, RequestHash: hash, AccountID: account, ProfileID: profile, SessionEpoch: 7, ClientExpiresAt: now.Add(time.Minute)})
		require.NoError(t, err)
		v := searchVerifier{t: t, mutate: mutate, v: searchprincipal.Verifier{Clock: func() time.Time { return now }, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }}}
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", op))
		called := false
		_, err = ProtectedUnaryInterceptor(v)(ctx, req, &grpc.UnaryServerInfo{FullMethod: matchmakingv1.MatchmakingService_StartSearch_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
			called = true
			subject, ok := searchprincipal.SubjectFromContext(ctx, req)
			require.True(t, ok)
			require.Equal(t, op, subject.SearchOperationID.String())
			require.Equal(t, account, subject.AccountID.String())
			require.Equal(t, profile, subject.ProfileID.String())
			return nil, nil
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
func TestOrdinaryStartSearchAlwaysDeniedBeforeHandler(t *testing.T) {
	called := false
	_, err := OrdinaryUnaryInterceptor()(context.Background(), &matchmakingv1.StartSearchRequest{}, &grpc.UnaryServerInfo{FullMethod: matchmakingv1.MatchmakingService_StartSearch_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}
func TestProtectedStartSearchMissingVerifierCannotFallBack(t *testing.T) {
	for _, v := range []verifier{nil, searchVerifier{t: t}} {
		called := false
		_, err := ProtectedUnaryInterceptor(v)(context.Background(), &matchmakingv1.StartSearchRequest{}, &grpc.UnaryServerInfo{FullMethod: matchmakingv1.MatchmakingService_StartSearch_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.Error(t, err)
		require.False(t, called)
	}
}
