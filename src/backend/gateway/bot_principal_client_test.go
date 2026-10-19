package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type verifiedBotLifecycleClient struct {
	botv1.BotServiceClient
	key         *rsa.PublicKey
	calls       int
	claims      tokenClaims
	enabledChat string
}

func withBotLifecycleTestActor(t *testing.T, tc *transcoder, request *http.Request) *http.Request {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	tc.lifecycleIssuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	tc.clients.botAuditAdmin = tc.clients.bot
	claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 1, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}
	return request.WithContext(context.WithValue(request.Context(), verifiedUserClaimsKey{}, claims))
}

func (f *verifiedBotLifecycleClient) verify(ctx context.Context, request proto.Message, method string) error {
	md, ok := metadata.FromOutgoingContext(ctx)
	if !ok || len(md) != 2 {
		return status.Error(codes.Unauthenticated, "unexpected initiating metadata")
	}
	transport, err := principal.IncomingMetadata(metadata.NewIncomingContext(context.Background(), md))
	if err != nil {
		return err
	}
	hash, err := principal.RequestHash(request)
	if err != nil {
		return err
	}
	_, err = principal.VerifyDelegatedUser(ctx, transport.BearerToken, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "bot", ExpectedRPC: method, ExpectedRequestID: transport.RequestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return f.key, nil }, SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
		if account != f.claims.UserID || epoch != f.claims.SessionEpoch {
			return status.Error(codes.Unauthenticated, "wrong verified actor")
		}
		return nil
	}})
	if err == nil {
		f.calls++
	}
	return err
}
func (f *verifiedBotLifecycleClient) DeleteBot(ctx context.Context, r *botv1.DeleteBotRequest, _ ...grpc.CallOption) (*botv1.DeleteBotResponse, error) {
	err := f.verify(ctx, r, botv1.BotService_DeleteBot_FullMethodName)
	return &botv1.DeleteBotResponse{}, err
}
func (f *verifiedBotLifecycleClient) InstallBotInSpace(ctx context.Context, r *botv1.InstallBotInSpaceRequest, _ ...grpc.CallOption) (*botv1.InstallBotInSpaceResponse, error) {
	err := f.verify(ctx, r, botv1.BotService_InstallBotInSpace_FullMethodName)
	return &botv1.InstallBotInSpaceResponse{InstallationId: uuid.NewString()}, err
}
func (f *verifiedBotLifecycleClient) UninstallBotFromSpace(ctx context.Context, r *botv1.UninstallBotFromSpaceRequest, _ ...grpc.CallOption) (*botv1.UninstallBotFromSpaceResponse, error) {
	err := f.verify(ctx, r, botv1.BotService_UninstallBotFromSpace_FullMethodName)
	return &botv1.UninstallBotFromSpaceResponse{}, err
}

func (f *verifiedBotLifecycleClient) SetBotChatEnabled(ctx context.Context, r *botv1.SetBotChatEnabledRequest, _ ...grpc.CallOption) (*botv1.SetBotChatEnabledResponse, error) {
	err := f.verify(ctx, r, botv1.BotService_SetBotChatEnabled_FullMethodName)
	if err == nil {
		f.enabledChat = r.GetChat().GetId()
	}
	return &botv1.SetBotChatEnabledResponse{}, err
}

type ordinaryEnableFallback struct {
	botv1.BotServiceClient
	calls int
}

func (f *ordinaryEnableFallback) SetBotChatEnabled(context.Context, *botv1.SetBotChatEnabledRequest, ...grpc.CallOption) (*botv1.SetBotChatEnabledResponse, error) {
	f.calls++
	return &botv1.SetBotChatEnabledResponse{}, nil
}

func TestServeBotEnabledUsesProtectedExactPathWithoutOrdinaryFallback(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 9, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}
	protected := &verifiedBotLifecycleClient{key: &key.PublicKey, claims: claims}
	ordinary := &ordinaryEnableFallback{}
	tc := newTranscoder(&grpcClients{bot: ordinary, botAuditAdmin: protected})
	tc.lifecycleIssuer = issuer
	chat := uuid.NewString()
	rest := uuid.NewString() + "/chats/" + chat + "/enabled"
	request := func(verified bool) *http.Request {
		r := httptest.NewRequest(http.MethodPatch, "/api/v1/bots/"+rest, strings.NewReader(`{"space_id":"`+uuid.NewString()+`","chat":{"id":"`+uuid.NewString()+`"},"enabled":true}`))
		if verified {
			r = r.WithContext(context.WithValue(r.Context(), verifiedUserClaimsKey{}, claims))
		}
		return r
	}
	r := request(true)
	require.True(t, isBotLifecycleRequest(r), "existing mutation requires strict Gateway authentication")
	response := httptest.NewRecorder()
	require.True(t, tc.serveBots(response, r, rest))
	require.Equal(t, http.StatusNoContent, response.Code, response.Body.String())
	require.Zero(t, ordinary.calls)
	require.Equal(t, 1, protected.calls)
	require.Equal(t, chat, protected.enabledChat, "path chat ID is authoritative before signing")
	response = httptest.NewRecorder()
	require.True(t, tc.serveBots(response, request(false), rest))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Zero(t, ordinary.calls)
	tc.clients.botAuditAdmin = nil
	response = httptest.NewRecorder()
	require.True(t, tc.serveBots(response, request(true), rest))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Zero(t, ordinary.calls)
}

func TestServeBotsLifecycleUsesProtectedVerifiedActorForAllThreeRoutes(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	claims := tokenClaims{UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 8, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}
	protected := &verifiedBotLifecycleClient{key: &key.PublicKey, claims: claims}
	tc := newTranscoder(&grpcClients{bot: &fakeBotClient{}, botAuditAdmin: protected})
	tc.lifecycleIssuer = issuer
	for _, test := range []struct {
		method, rest, body string
		status             int
	}{{http.MethodPost, uuid.NewString() + "/spaces/" + uuid.NewString() + "/install", `{"acknowledgePrivilegedScopes":true}`, http.StatusOK}, {http.MethodDelete, uuid.NewString() + "/spaces/" + uuid.NewString(), "", http.StatusNoContent}, {http.MethodDelete, uuid.NewString(), "", http.StatusNoContent}} {
		request := httptest.NewRequest(test.method, "/api/v1/bots/"+test.rest, strings.NewReader(test.body))
		request = request.WithContext(context.WithValue(request.Context(), verifiedUserClaimsKey{}, claims))
		request.Header.Set("X-Voice-User-Id", uuid.NewString())
		request.Header.Set("X-Voice-Profile-Id", uuid.NewString())
		response := httptest.NewRecorder()
		require.True(t, tc.serveBots(response, request, test.rest))
		require.Equal(t, test.status, response.Code, response.Body.String())
	}
	require.Equal(t, 3, protected.calls)
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/bots/"+uuid.NewString(), nil)
	request.Header.Set("X-Voice-User-Id", claims.UserID)
	request.Header.Set("X-Voice-Profile-Id", claims.ProfileID)
	response := httptest.NewRecorder()
	require.True(t, tc.serveBots(response, request, uuid.NewString()))
	require.Equal(t, http.StatusUnauthorized, response.Code)
	require.Equal(t, 3, protected.calls)
	tc.clients.botAuditAdmin = nil
	request = request.WithContext(context.WithValue(request.Context(), verifiedUserClaimsKey{}, claims))
	response = httptest.NewRecorder()
	require.True(t, tc.serveBots(response, request, uuid.NewString()))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
}
