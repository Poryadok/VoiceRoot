package gameprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	"voice/backend/pkg/principal"
)

func TestProvisionInterceptor_BindsIssuerAudienceMethodRequestHashAndOperationID(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: "gis-key", PrivateKey: key})
	require.NoError(t, err)
	verifier := &Verifier{
		Resolve: func(_ context.Context, gotIssuer, keyID string) (*rsa.PublicKey, error) {
			require.Equal(t, "gameintegration", gotIssuer)
			require.Equal(t, "gis-key", keyID)
			return &key.PublicKey, nil
		}, Replay: newTestReplayGuard().record,
	}
	interceptor := UnaryServerInterceptor(verifier)
	request := validProvisionRequest()
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)

	cases := []struct {
		name   string
		issuer string
		aud    string
		rpc    string
		rid    string
		hash   string
	}{
		{name: "wrong issuer", issuer: "chat", aud: "voice", rpc: ProvisionMethod, rid: request.OperationId, hash: requestHash},
		{name: "wrong audience", issuer: "gameintegration", aud: "chat", rpc: ProvisionMethod, rid: request.OperationId, hash: requestHash},
		{name: "wrong method", issuer: "gameintegration", aud: "voice", rpc: "/voice.calls.v1.VoiceService/StartCall", rid: request.OperationId, hash: requestHash},
		{name: "wrong request hash", issuer: "gameintegration", aud: "voice", rpc: ProvisionMethod, rid: request.OperationId, hash: "sha256:" + strings.Repeat("0", 64)},
		{name: "wrong request id", issuer: "gameintegration", aud: "voice", rpc: ProvisionMethod, rid: "00000000-0000-4000-8000-000000000001", hash: requestHash},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tokenIssuer := issuer
			if tc.issuer != "gameintegration" {
				tokenIssuer, err = principal.NewIssuer(principal.IssuerConfig{Issuer: tc.issuer, KeyID: "gis-key", PrivateKey: key})
				require.NoError(t, err)
			}
			token := issueGameServiceToken(t, tokenIssuer, tc.aud, tc.rpc, tc.rid, tc.hash)
			requestID := request.OperationId
			if tc.name == "wrong request id" {
				requestID = tc.rid
			}
			ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", requestID))
			called := false
			_, callErr := interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: ProvisionMethod}, func(context.Context, any) (any, error) {
				called = true
				return &callsv1.ProvisionGameSessionRoomResponse{}, nil
			})
			require.False(t, called)
			require.Equal(t, codes.Unauthenticated, status.Code(callErr))
		})
	}
}

func TestProvisionInterceptor_RejectsJTIReplayAndUnallowlistedMethods(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: "gis-key", PrivateKey: key})
	require.NoError(t, err)
	replay := newTestReplayGuard()
	interceptor := UnaryServerInterceptor(&Verifier{Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: replay.record})
	request := validProvisionRequest()
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token := issueGameServiceToken(t, issuer, "voice", ProvisionMethod, request.OperationId, requestHash)

	invoke := func(method string) error {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
		_, callErr := interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
			return &callsv1.ProvisionGameSessionRoomResponse{}, nil
		})
		return callErr
	}
	require.NoError(t, invoke(ProvisionMethod))
	require.Equal(t, codes.Unauthenticated, status.Code(invoke(ProvisionMethod)), "same JTI cannot authorize a second attempt")
	require.Equal(t, codes.PermissionDenied, status.Code(invoke("/voice.calls.v1.GameSessionProvisioningService/Other")), "only the exact method is exposed")
}

func TestProvisionInterceptor_RejectsUnknownProtobufFields(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: "gis-key", PrivateKey: key})
	require.NoError(t, err)
	replay := newTestReplayGuard()
	interceptor := UnaryServerInterceptor(&Verifier{Resolve: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: replay.record})
	request := validProvisionRequest()
	request.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token := issueGameServiceToken(t, issuer, "voice", ProvisionMethod, request.OperationId, hash)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
	called := false
	_, err = interceptor(ctx, request, &grpc.UnaryServerInfo{FullMethod: ProvisionMethod}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.False(t, called)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCloseInterceptorBindsGISPrincipalToExactCloseRequest(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gameintegration", KeyID: "gis-key", PrivateKey: key})
	require.NoError(t, err)
	verifier := &Verifier{
		Resolve: func(_ context.Context, gotIssuer, keyID string) (*rsa.PublicKey, error) {
			require.Equal(t, "gameintegration", gotIssuer)
			require.Equal(t, "gis-key", keyID)
			return &key.PublicKey, nil
		}, Replay: newTestReplayGuard().record,
	}
	request := validCloseRequest()
	requestHash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token := issueGameServiceToken(t, issuer, "voice", CloseMethod, request.OperationId, requestHash)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"authorization", "Bearer "+token, "x-request-id", request.OperationId,
	))
	called := false
	_, err = UnaryServerInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: CloseMethod}, func(verified context.Context, received any) (any, error) {
		called = true
		require.NoError(t, RequireClose(verified, received.(*callsv1.CloseGameSessionRoomRequest)))
		return nil, nil
	})
	require.NoError(t, err)
	require.True(t, called)
	require.True(t, AllowsMethod(CloseMethod))
	require.False(t, AllowsMethod("/voice.calls.v1.GameSessionProvisioningService/Other"))

	_, err = UnaryServerInterceptor(verifier)(ctx, request, &grpc.UnaryServerInfo{FullMethod: ProvisionMethod}, func(context.Context, any) (any, error) {
		t.Fatal("close assertion must not authorize provisioning")
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestGamePrincipalConfig_FailsClosedWhenPartiallyConfigured(t *testing.T) {
	t.Setenv("VOICE_GAME_PRINCIPAL_GRPC_LISTEN", ":9191")
	t.Setenv("VOICE_GAME_PRINCIPAL_TLS_CERT_FILE", "")
	_, enabled, err := LoadFromEnv()
	require.True(t, enabled)
	require.Error(t, err, "partial private listener configuration must fail startup")
}

func TestGamePrincipalConfig_DisablesOnlyWhenEveryOwnedSettingIsAbsent(t *testing.T) {
	for _, name := range ownedEnvNames {
		t.Setenv(name, "")
	}
	_, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.False(t, enabled)
}

type testReplayGuard struct {
	mu   sync.Mutex
	seen map[string]bool
}

func newTestReplayGuard() *testReplayGuard { return &testReplayGuard{seen: map[string]bool{}} }
func (r *testReplayGuard) record(_ context.Context, issuer, jti string, _ time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := issuer + ":" + jti
	if r.seen[key] {
		return context.Canceled
	}
	r.seen[key] = true
	return nil
}

func issueGameServiceToken(t *testing.T, issuer *principal.Issuer, audience, rpc, requestID, hash string) string {
	t.Helper()
	token, err := issuer.IssueService(principal.ServiceInput{Audience: audience, RPC: rpc, RequestID: requestID, RequestHash: hash})
	require.NoError(t, err)
	return token
}

func validProvisionRequest() *callsv1.ProvisionGameSessionRoomRequest {
	return &callsv1.ProvisionGameSessionRoomRequest{
		OperationId: "00000000-0000-4000-8000-000000000011", ApplicationId: "00000000-0000-4000-8000-000000000012",
		EnvironmentId: "00000000-0000-4000-8000-000000000013", SessionId: "00000000-0000-4000-8000-000000000016",
		Resource: &callsv1.GameSessionResourceRef{Kind: callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH, ExternalResourceKey: "opaque"},
		ChatId:   "00000000-0000-4000-8000-000000000014", ChatCreationOperationId: "00000000-0000-4000-8000-000000000015",
	}
}

func validCloseRequest() *callsv1.CloseGameSessionRoomRequest {
	return &callsv1.CloseGameSessionRoomRequest{
		OperationId: "00000000-0000-4000-8000-000000000021", ApplicationId: "00000000-0000-4000-8000-000000000012",
		EnvironmentId: "00000000-0000-4000-8000-000000000013", SessionId: "00000000-0000-4000-8000-000000000016",
		Resource: &callsv1.GameSessionResourceRef{Kind: callsv1.GameSessionResourceKind_GAME_SESSION_RESOURCE_KIND_MATCH, ExternalResourceKey: "opaque"},
		ChatId:   "00000000-0000-4000-8000-000000000014", ChatCreationOperationId: "00000000-0000-4000-8000-000000000015",
	}
}
