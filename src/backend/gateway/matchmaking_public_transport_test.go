package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	matchmakingv1 "voice.app/voice/matchmaking/v1"
	"voice/backend/pkg/principal"
)

type verifyingPublicMatchmaking struct {
	matchmakingv1.MatchmakingServiceClient
	verify func(context.Context, proto.Message, string, string)
	calls  int
}

func (c *verifyingPublicMatchmaking) StartSearch(ctx context.Context, r *matchmakingv1.StartSearchRequest, _ ...grpc.CallOption) (*matchmakingv1.StartSearchResponse, error) {
	c.calls++
	c.verify(ctx, r, matchmakingv1.MatchmakingService_StartSearch_FullMethodName, r.OperationId)
	return &matchmakingv1.StartSearchResponse{}, nil
}
func (c *verifyingPublicMatchmaking) RespondToMatch(ctx context.Context, r *matchmakingv1.RespondToMatchRequest, _ ...grpc.CallOption) (*matchmakingv1.RespondToMatchResponse, error) {
	c.calls++
	c.verify(ctx, r, matchmakingv1.MatchmakingService_RespondToMatch_FullMethodName, r.OperationId)
	return &matchmakingv1.RespondToMatchResponse{}, nil
}

func TestProtectedPublicMatchmakingDelegatesExactStableOperationWithFreshJWT(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, profile, match, op := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, actorType := range []string{"regular", "guest"} {
		t.Run(actorType, func(t *testing.T) {
			client := &verifyingPublicMatchmaking{}
			seen := map[string]bool{}
			client.verify = func(ctx context.Context, req proto.Message, method, id string) {
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				require.Len(t, md, 2, "strip raw actor metadata and inherited outgoing metadata")
				require.Equal(t, []string{op}, md.Get("x-request-id"))
				require.Equal(t, op, id)
				hash, e := principal.RequestHash(req)
				require.NoError(t, e)
				v, e := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
					ExpectedIssuer: "gateway", ExpectedAudience: "matchmaking", ExpectedRPC: method, ExpectedRequestID: op, ExpectedRequestHash: hash,
					KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
					ReplayGuard: func(_ context.Context, _ string, id string, _ time.Time) error {
						require.False(t, seen[id], "retry gets fresh JWT")
						seen[id] = true
						return nil
					},
					SessionEpochChecker: func(_ context.Context, a string, e int64) error {
						require.Equal(t, account, a)
						require.Equal(t, int64(7), e)
						return nil
					},
				})
				require.NoError(t, e)
				require.Equal(t, profile, v.ProfileID)
				require.Equal(t, actorType, v.AccountType)
			}
			h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"actor": {UserID: account, ProfileID: profile, SessionEpoch: 7, AccountType: actorType, AuthenticatedAccountType: actorType, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: grpcClients{matchmakingComplete: client}, matchmakingCompleteIssuer: issuer}})
			for _, accept := range []bool{false, true} {
				for range 2 {
					rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/"+match+"/respond", fmt.Sprintf(`{"accept":%t,"operationId":%q}`, accept, op), map[string]string{"Authorization": "Bearer actor", "Content-Type": "application/json", "X-Voice-Profile-Id": uuid.NewString()})
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				}
			}
			rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/search", fmt.Sprintf(`{"gameId":%q,"mode":"duo","criteriaJson":"{}","operationId":%q}`, uuid.NewString(), op), map[string]string{"Authorization": "Bearer actor", "Content-Type": "application/json"})
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			require.Equal(t, 5, client.calls)
		})
	}
}

func TestProtectedPublicMatchmakingRejectsAmbiguousBodiesBeforeDispatch(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	match, op := uuid.NewString(), uuid.NewString()
	client := &verifyingPublicMatchmaking{verify: func(context.Context, proto.Message, string, string) { t.Fatal("invalid body dispatched") }}
	h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"actor": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: grpcClients{matchmakingComplete: client}, matchmakingCompleteIssuer: issuer}})
	bodies := []string{
		fmt.Sprintf(`{"accept":true,"accept":false,"operationId":%q}`, op),
		fmt.Sprintf(`{"accept":true,"operationId":%q,"operation_id":%q}`, op, op),
		fmt.Sprintf(`{"accept":true,"operationId":%q,"profileId":%q}`, op, uuid.NewString()),
		fmt.Sprintf(`{"accept":true,"operationId":%q,"matchId":%q}`, op, uuid.NewString()),
		`{"accept":true}`, fmt.Sprintf(`{"operationId":%q}`, op),
	}
	for _, body := range bodies {
		rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/"+match+"/respond", body, map[string]string{"Authorization": "Bearer actor", "Content-Type": "application/json"})
		require.Equal(t, http.StatusBadRequest, rec.Code, body+rec.Body.String())
	}
	require.Zero(t, client.calls)
}

func TestProtectedPublicMatchmakingNeverFallsBackToOrdinary(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	match, op := uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name                         string
		missingClient, missingIssuer bool
		actorType                    string
		expected                     int
	}{
		{"missing-client", true, false, "regular", http.StatusServiceUnavailable},
		{"missing-issuer", false, true, "regular", http.StatusServiceUnavailable},
		{"missing-authenticated-type", false, false, "", http.StatusForbidden},
		{"bot", false, false, "bot", http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spy := &verifyingPublicMatchmaking{verify: func(context.Context, proto.Message, string, string) { t.Fatal("invalid native composition dispatched") }}
			clients := grpcClients{matchmaking: spy, matchmakingComplete: spy}
			signer := issuer
			if tc.missingClient {
				clients.matchmakingComplete = nil
			}
			if tc.missingIssuer {
				signer = nil
			}
			h := newGatewayForContract(t, gatewayTestOptions{tokenClaims: map[string]tokenClaims{"actor": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AccountType: "regular", AuthenticatedAccountType: tc.actorType, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: clients, matchmakingCompleteIssuer: signer}})
			rec := performRequest(h, http.MethodPost, "/api/v1/matchmaking/matches/"+match+"/respond", fmt.Sprintf(`{"accept":true,"operationId":%q}`, op), map[string]string{"Authorization": "Bearer actor", "Content-Type": "application/json"})
			require.Equal(t, tc.expected, rec.Code, rec.Body.String())
			require.Zero(t, spy.calls)
		})
	}
}
