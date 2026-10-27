package main

import (
	"context"
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
	chatv1 "voice.app/voice/chat/v1"
	"voice/backend/pkg/principal"
)

type catalogGatewayClient struct {
	chatv1.CatalogAssetServiceClient
	resolve func(context.Context, *chatv1.ResolveGifAssetRequest) (*chatv1.ResolveGifAssetResponse, error)
}

func (c catalogGatewayClient) ResolveGifAsset(ctx context.Context, r *chatv1.ResolveGifAssetRequest, _ ...grpc.CallOption) (*chatv1.ResolveGifAssetResponse, error) {
	return c.resolve(ctx, r)
}
func catalogSelectionRequest(t *testing.T, cfg gatewayConfig, f *creatorFixture, payload map[string]any, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/gifs/resolve", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, payload))
	r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	r.Header.Set("X-Request-Id", "untrusted")
	w := httptest.NewRecorder()
	newGateway(cfg).ServeHTTP(w, r)
	return w
}
func TestCatalogSelectionProtectedDispatch(t *testing.T) {
	f := newCreatorFixture(t)
	cfg := f.config(nil)
	op, dest, file, source := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	calls := 0
	var ids, tokens []string
	want := &chatv1.ResolveGifAssetRequest{ProtocolVersion: 1, OperationId: op, Provider: "fixture", ProviderId: "one", DestinationChat: &chatv1.ChatRef{Id: dest}}
	cfg.transcoder.clients.chatCatalog = catalogGatewayClient{resolve: func(ctx context.Context, req *chatv1.ResolveGifAssetRequest) (*chatv1.ResolveGifAssetResponse, error) {
		calls++
		require.True(t, proto.Equal(want, req))
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		rid := md.Get("x-request-id")[0]
		require.True(t, canonicalLifecycleUUID(rid))
		require.NotEqual(t, op, rid)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		actor, err := principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "chat", ExpectedRPC: chatv1.CatalogAssetService_ResolveGifAsset_FullMethodName, ExpectedRequestID: rid, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }})
		require.NoError(t, err)
		require.Equal(t, f.payload["user_id"], actor.AccountID)
		require.Equal(t, f.payload["profile_id"], actor.ProfileID)
		require.EqualValues(t, 7, actor.SessionEpoch)
		ids = append(ids, rid)
		tokens = append(tokens, token)
		return &chatv1.ResolveGifAssetResponse{SourceId: source, FileId: file, Provider: req.Provider, ProviderId: req.ProviderId, Status: "processing"}, nil
	}}
	body := `{"protocol_version":1,"operation_id":"` + op + `","provider":"fixture","provider_id":"one","destination_chat":{"id":"` + dest + `"}}`
	for range 2 {
		w := catalogSelectionRequest(t, cfg, f, f.payload, body)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
		require.Contains(t, w.Body.String(), `"file_id":"`+file+`"`)
	}
	require.Equal(t, 2, calls)
	require.NotEqual(t, ids[0], ids[1])
	require.NotEqual(t, tokens[0], tokens[1])
}
func TestCatalogSelectionRejectsInvalidBodyAndMissingTransport(t *testing.T) {
	f := newCreatorFixture(t)
	body := `{"protocol_version":1,"operation_id":"` + uuid.NewString() + `","provider":"fixture","provider_id":"one","destination_chat":{"id":"` + uuid.NewString() + `"}}`
	for _, scenario := range []string{"unknown actor", "client URL", "invalid op", "invalid destination", "oversized", "missing transcoder", "missing protected", "upstream unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := f.config(nil)
			calls, proxy := 0, 0
			cfg.restUpstreams = map[string]http.Handler{"gifs": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxy++ })}
			cfg.transcoder.clients.chatCatalog = catalogGatewayClient{resolve: func(context.Context, *chatv1.ResolveGifAssetRequest) (*chatv1.ResolveGifAssetResponse, error) {
				calls++
				return nil, status.Error(codes.Unavailable, "private-source-url")
			}}
			payload := body
			want := http.StatusBadRequest
			switch scenario {
			case "unknown actor":
				payload = strings.TrimSuffix(body, "}") + `,"actor_profile_id":"` + uuid.NewString() + `"}`
			case "client URL":
				payload = strings.TrimSuffix(body, "}") + `,"source_url":"https://attacker.test/a.gif"}`
			case "invalid op":
				payload = strings.Replace(body, `"operation_id":"`, `"operation_id":"bad`, 1)
			case "invalid destination":
				payload = strings.Replace(body, `"destination_chat":{"id":"`, `"destination_chat":{"id":"bad`, 1)
			case "oversized":
				payload = `{"provider":"` + strings.Repeat("x", 65536) + `"}`
			case "missing transcoder":
				cfg.transcoder = nil
				want = http.StatusServiceUnavailable
			case "missing protected":
				cfg.transcoder.clients.chatCatalog = nil
				want = http.StatusServiceUnavailable
			case "upstream unavailable":
				want = http.StatusServiceUnavailable
			}
			w := catalogSelectionRequest(t, cfg, f, f.payload, payload)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, proxy)
			if scenario != "upstream unavailable" {
				require.Zero(t, calls)
			}
			require.NotContains(t, w.Body.String(), "private-source-url")
		})
	}
}
func TestCatalogSelectionCannotMintFromStaticOrStaleAuth(t *testing.T) {
	f := newCreatorFixture(t)
	for _, scenario := range []string{"static", "missing profile", "missing jti", "stale epoch", "missing account type", "compat epoch"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := f.config(nil)
			payload := cloneConfigJWTPayload(f.payload)
			calls := 0
			cfg.transcoder.clients.chatCatalog = catalogGatewayClient{resolve: func(context.Context, *chatv1.ResolveGifAssetRequest) (*chatv1.ResolveGifAssetResponse, error) {
				calls++
				return &chatv1.ResolveGifAssetResponse{}, nil
			}}
			want := http.StatusUnauthorized
			switch scenario {
			case "static":
				token := signConfigJWT(t, "creator-key", f.key, payload)
				cfg.tokenValidator = staticTokenValidator{token: {UserID: payload["user_id"].(string), ProfileID: payload["profile_id"].(string), SessionEpoch: 7, AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}
			case "missing profile":
				delete(payload, "profile_id")
			case "missing jti":
				delete(payload, "jti")
			case "stale epoch":
				payload["session_epoch"] = 6
			case "missing account type":
				delete(payload, "account_type")
			case "compat epoch":
				cfg.sessionEpochStrict = false
				want = http.StatusServiceUnavailable
			}
			w := catalogSelectionRequest(t, cfg, f, payload, `{}`)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, calls)
		})
	}
}

func TestCatalogSelectionResponseQualificationAndRateLimit(t *testing.T) {
	f := newCreatorFixture(t)
	body := `{"protocol_version":1,"operation_id":"` + uuid.NewString() + `","provider":"fixture","provider_id":"one","destination_chat":{"id":"` + uuid.NewString() + `"}}`
	for _, scenario := range []string{"nil", "wrong provider", "wrong result", "invalid source", "invalid file", "unknown status", "unknown response", "rate exhausted"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := f.config(nil)
			cfg.transcoder.clients.chatCatalog = catalogGatewayClient{resolve: func(context.Context, *chatv1.ResolveGifAssetRequest) (*chatv1.ResolveGifAssetResponse, error) {
				r := &chatv1.ResolveGifAssetResponse{SourceId: uuid.NewString(), FileId: uuid.NewString(), Provider: "fixture", ProviderId: "one", Status: "ready"}
				switch scenario {
				case "nil":
					return nil, nil
				case "wrong provider":
					r.Provider = "other"
				case "wrong result":
					r.ProviderId = "other"
				case "invalid source":
					r.SourceId = "bad"
				case "invalid file":
					r.FileId = ""
				case "unknown status":
					r.Status = "approved"
				case "unknown response":
					r.ProtoReflect().SetUnknown([]byte{0x98, 0x06, 0x01})
				case "rate exhausted":
					return nil, status.Error(codes.ResourceExhausted, "provider-private-secret")
				}
				return r, nil
			}}
			w := catalogSelectionRequest(t, cfg, f, f.payload, body)
			want := http.StatusServiceUnavailable
			if scenario == "rate exhausted" {
				want = http.StatusTooManyRequests
			}
			require.Equal(t, want, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), "private-secret")
		})
	}
}
func TestCatalogSelectionConfigRejectsPartialOrExplicitEmpty(t *testing.T) {
	for _, suffix := range []string{"GRPC_ADDR", "TLS_CA_FILE", "TLS_SERVER_NAME", "CLIENT_CERT_FILE", "CLIENT_KEY_FILE"} {
		t.Setenv("GATEWAY_CHAT_CATALOG_ASSET_"+suffix, "")
	}
	_, enabled, err := catalogAssetClientConfigFromEnv()
	require.True(t, enabled)
	require.Error(t, err)
	t.Setenv("GATEWAY_CHAT_CATALOG_ASSET_GRPC_ADDR", "private.test:9099")
	_, enabled, err = catalogAssetClientConfigFromEnv()
	require.True(t, enabled)
	require.Error(t, err)
}
