package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"net/http"
	"strings"
	"testing"
	"time"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type verifyingTerminalMatchmaking struct{ *verifyingPublicMatchmaking }

func (c *verifyingTerminalMatchmaking) terminal(ctx context.Context, req proto.Message, method string) {
	md, _ := metadata.FromOutgoingContext(ctx)
	c.calls++
	c.verify(ctx, req, method, firstNativeMetadata(md.Get("x-request-id")))
}
func firstNativeMetadata(values []string) string {
	if len(values) != 1 {
		return ""
	}
	return values[0]
}
func (c *verifyingTerminalMatchmaking) GetMatch(ctx context.Context, r *matchmakingv1.GetMatchRequest, _ ...grpc.CallOption) (*matchmakingv1.GetMatchResponse, error) {
	c.terminal(ctx, r, matchmakingv1.MatchmakingService_GetMatch_FullMethodName)
	return &matchmakingv1.GetMatchResponse{}, nil
}
func (c *verifyingTerminalMatchmaking) GetSearchStatus(ctx context.Context, r *matchmakingv1.GetSearchStatusRequest, _ ...grpc.CallOption) (*matchmakingv1.GetSearchStatusResponse, error) {
	c.terminal(ctx, r, matchmakingv1.MatchmakingService_GetSearchStatus_FullMethodName)
	return &matchmakingv1.GetSearchStatusResponse{}, nil
}
func (c *verifyingTerminalMatchmaking) CancelSearch(ctx context.Context, r *matchmakingv1.CancelSearchRequest, _ ...grpc.CallOption) (*matchmakingv1.CancelSearchResponse, error) {
	c.terminal(ctx, r, matchmakingv1.MatchmakingService_CancelSearch_FullMethodName)
	return &matchmakingv1.CancelSearchResponse{}, nil
}
func (c *verifyingTerminalMatchmaking) GetMatchHistory(ctx context.Context, r *matchmakingv1.GetMatchHistoryRequest, _ ...grpc.CallOption) (*matchmakingv1.GetMatchHistoryResponse, error) {
	c.terminal(ctx, r, matchmakingv1.MatchmakingService_GetMatchHistory_FullMethodName)
	return &matchmakingv1.GetMatchHistoryResponse{}, nil
}

func TestProtectedTerminalMatchmakingRoutesBindVerifiedActorWithoutOrdinaryClient(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, profile, match, session := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	seen := map[string]bool{}
	client := &verifyingTerminalMatchmaking{&verifyingPublicMatchmaking{}}
	client.verify = func(ctx context.Context, req proto.Message, method, id string) {
		require.True(t, canonicalLifecycleUUID(id))
		md, _ := metadata.FromOutgoingContext(ctx)
		require.Len(t, md, 2)
		hash, e := principal.RequestHash(req)
		require.NoError(t, e)
		actor, e := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "matchmaking", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error {
			require.False(t, seen[jti])
			seen[jti] = true
			return nil
		}, SessionEpochChecker: func(context.Context, string, int64) error { return nil }})
		require.NoError(t, e)
		require.Equal(t, account, actor.AccountID)
		require.Equal(t, profile, actor.ProfileID)
	}
	h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"actor": {UserID: account, ProfileID: profile, AccountType: "regular", AuthenticatedAccountType: "regular", SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: grpcClients{matchmakingComplete: client}, matchmakingCompleteIssuer: issuer}})
	for _, r := range []struct{ method, path string }{{http.MethodGet, "matches/" + match}, {http.MethodGet, "search/" + session}, {http.MethodDelete, "search/" + session}, {http.MethodGet, "profile/me/matches"}} {
		rec := performRequest(h, r.method, "/api/v1/matchmaking/"+r.path, "", map[string]string{"Authorization": "Bearer actor", "X-Voice-Profile-Id": uuid.NewString()})
		require.Equal(t, http.StatusOK, rec.Code, r.path+rec.Body.String())
	}
	require.Equal(t, 4, client.calls)
	rec := performRequest(h, http.MethodDelete, "/api/v1/matchmaking/search/not-an-id", "", map[string]string{"Authorization": "Bearer actor"})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, 4, client.calls)
}
