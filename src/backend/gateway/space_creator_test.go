package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	spacev1 "voice.app/voice/space/v1"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/principal"
)

type creatorClient struct {
	spacev1.SpaceServiceClient
	calls int
	call  func(context.Context, *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error)
}

func (c *creatorClient) CreateSpace(ctx context.Context, req *spacev1.CreateSpaceRequest, _ ...grpc.CallOption) (*spacev1.CreateSpaceResponse, error) {
	c.calls++
	if c.call != nil {
		return c.call(ctx, req)
	}
	return &spacev1.CreateSpaceResponse{Space: &spacev1.Space{Id: "created", Name: req.Name, Description: req.Description}}, nil
}

type creatorFixture struct {
	key       *rsa.PrivateKey
	issuer    *principal.Issuer
	validator *voicejwt.Validator
	payload   map[string]any
	redis     *miniredis.Miniredis
	jwksURL   string
}

func newCreatorFixture(t *testing.T) *creatorFixture {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{configRSAJWK("creator-key", &key.PublicKey)}}))
	}))
	t.Cleanup(jwks.Close)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "creator-key", PrivateKey: key})
	require.NoError(t, err)
	r := miniredis.RunT(t)
	account, profile := uuid.NewString(), uuid.NewString()
	r.Set("auth:session:min_epoch:"+account, "7")
	return &creatorFixture{key, issuer, voicejwt.NewJWKSValidator(jwks.URL, "voice-auth", "voice-client"),
		map[string]any{"sub": account, "user_id": account, "profile_id": profile, "session_epoch": int64(7), "account_type": "regular", "iss": "voice-auth", "aud": "voice-client", "jti": uuid.NewString(), "exp": time.Now().Add(time.Minute).Unix()}, r, jwks.URL}
}

func (f *creatorFixture) config(client spacev1.SpaceServiceClient) gatewayConfig {
	return gatewayConfig{tokenValidator: f.validator, tokenBlacklist: newRedisTokenBlacklist(f.redis.Addr(), "", ""),
		blacklistRequired: true, sessionEpochStrict: true, sessionEpochFloor: newRedisSessionEpochFloor(f.redis.Addr(), ""),
		transcoder: &transcoder{clients: grpcClients{space: &creatorClient{}, spaceLifecycle: client}, lifecycleIssuer: f.issuer}}
}

func creatorRequest(t *testing.T, h http.Handler, f *creatorFixture, payload map[string]any, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/spaces", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, payload))
	r.Header.Set("X-Voice-User-Id", uuid.NewString())
	r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	r.Header.Set("X-Voice-Account-Type", "regular")
	r.Header.Set("X-Voice-Session-Epoch", "99")
	r.Header.Set("X-Request-Id", "untrusted-client-id")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSpaceCreatorProtectedDispatch(t *testing.T) {
	f := newCreatorFixture(t)
	client := &creatorClient{}
	var tokens, requestIDs []string
	client.call = func(ctx context.Context, req *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Len(t, md, 2)
		require.Len(t, md.Get("authorization"), 1)
		require.Len(t, md.Get("x-request-id"), 1)
		requestID := md.Get("x-request-id")[0]
		require.True(t, canonicalLifecycleUUID(requestID))
		token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		cfg := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_CreateSpace_FullMethodName, ExpectedRequestID: requestID, ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
			ReplayGuard: func(context.Context, string, string, time.Time) error { return nil },
			SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
				require.Equal(t, f.payload["user_id"], account)
				require.EqualValues(t, 7, epoch)
				return nil
			}}
		p, err := principal.VerifyDelegatedUser(ctx, token, cfg)
		require.NoError(t, err)
		require.Equal(t, f.payload["profile_id"], p.ProfileID)
		// Inspect the exact signed claim; compatibility defaults must not satisfy it.
		parts := strings.Split(token, ".")
		require.Len(t, parts, 3)
		require.Equal(t, "regular", p.AccountType)
		require.LessOrEqual(t, p.ExpiresAt.Sub(p.IssuedAt), 30*time.Second)
		require.LessOrEqual(t, p.ExpiresAt.Unix(), f.payload["exp"].(int64))
		for _, mutate := range []func(*principal.VerifyConfig){
			func(c *principal.VerifyConfig) { c.ExpectedAudience = "role" },
			func(c *principal.VerifyConfig) { c.ExpectedRPC = spacev1.SpaceService_DeleteSpace_FullMethodName },
			func(c *principal.VerifyConfig) { c.ExpectedRequestID = uuid.NewString() },
			func(c *principal.VerifyConfig) {
				changed := proto.Clone(req).(*spacev1.CreateSpaceRequest)
				changed.Description += "tampered"
				c.ExpectedRequestHash, _ = principal.RequestHash(changed)
			},
		} {
			bad := cfg
			mutate(&bad)
			_, err := principal.VerifyDelegatedUser(ctx, token, bad)
			require.Error(t, err)
		}
		tokens = append(tokens, token)
		requestIDs = append(requestIDs, requestID)
		return &spacev1.CreateSpaceResponse{Space: &spacev1.Space{Id: "created", Name: req.Name, Description: req.Description}}, nil
	}
	cfg := f.config(client)
	// A protected-only configuration must not depend on an ordinary Space client.
	cfg.transcoder.clients.space = nil
	for i := 0; i < 2; i++ {
		if i == 1 {
			f.payload["exp"] = time.Now().Add(10 * time.Second).Unix()
			token := signConfigJWT(t, "creator-key", f.key, f.payload)
			cfg.tokenValidator = chainedTokenValidator{static: staticTokenValidator{
				token: {UserID: uuid.NewString(), ProfileID: uuid.NewString(), AccountType: "regular", SessionEpoch: 99, ExpiresAt: time.Now().Add(time.Hour)},
			}, next: f.validator}
		}
		h := newGateway(cfg)
		w := creatorRequest(t, h, f, f.payload, `{"name":"Friday squad","description":"exact body"}`)
		require.Equal(t, 200, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), `"name":"Friday squad"`)
	}
	require.Equal(t, 2, client.calls)
	require.NotEqual(t, tokens[0], tokens[1])
	require.NotEqual(t, requestIDs[0], requestIDs[1])
}

func TestSpaceCreatorDeniesUntrustedInputs(t *testing.T) {
	f := newCreatorFixture(t)
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
		body string
		code int
	}{
		{"missing type", func(p map[string]any) { delete(p, "account_type") }, "", 403},
		{"blank type", func(p map[string]any) { p["account_type"] = "" }, "", 403},
		{"padded regular", func(p map[string]any) { p["account_type"] = " regular " }, "", 403},
		{"guest", func(p map[string]any) { p["account_type"] = "guest" }, "", 403},
		{"sdk", func(p map[string]any) { p["account_type"] = "sdk-account" }, "", 403},
		{"bot", func(p map[string]any) { p["account_type"] = "bot" }, "", 403},
		{"nil account", func(p map[string]any) { p["user_id"] = uuid.Nil.String() }, "", 401},
		{"nil profile", func(p map[string]any) { p["profile_id"] = uuid.Nil.String() }, "", 401},
		{"missing profile", func(p map[string]any) { delete(p, "profile_id") }, "", 401},
		{"uppercase profile", func(p map[string]any) { p["profile_id"] = strings.ToUpper(uuid.NewString()) }, "", 401},
		{"missing epoch", func(p map[string]any) { delete(p, "session_epoch") }, "", 401},
		{"missing jti", func(p map[string]any) { delete(p, "jti") }, "", 401},
		{"expired", func(p map[string]any) { p["exp"] = time.Now().Add(-time.Minute).Unix() }, "", 401},
		{"stale epoch", func(p map[string]any) { p["session_epoch"] = 6 }, "", 401},
		{"wrong issuer", func(p map[string]any) { p["iss"] = "other" }, "", 401},
		{"wrong audience", func(p map[string]any) { p["aud"] = "other" }, "", 401},
		{"unknown owner", nil, `{"name":"n","owner_profile_id":"spoof"}`, 400},
		{"unknown field", nil, `{"name":"n","actor":"spoof"}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := cloneConfigJWTPayload(f.payload)
			if tc.edit != nil {
				tc.edit(payload)
			}
			client := &creatorClient{}
			body := tc.body
			if body == "" {
				body = `{"name":"n"}`
			}
			w := creatorRequest(t, newGateway(f.config(client)), f, payload, body)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			require.Zero(t, client.calls)
		})
	}
}

func TestSpaceCreatorAuthenticationFailures(t *testing.T) {
	f := newCreatorFixture(t)
	for _, mode := range []string{"signature", "unknown kid", "revoked", "missing floor", "corrupt floor", "duplicate bearer", "query token", "unbound issuer", "unbound audience"} {
		t.Run(mode, func(t *testing.T) {
			client := &creatorClient{}
			token := signConfigJWT(t, "creator-key", f.key, f.payload)
			want := 401
			switch mode {
			case "signature":
				parts := strings.Split(token, ".")
				// Replace the whole signature with canonical, invalid bytes.
				parts[2] = "AA"
				token = strings.Join(parts, ".")
			case "unknown kid":
				token = signConfigJWT(t, "absent-key", f.key, f.payload)
			case "revoked":
				f.redis.Set("jwt:blacklist:"+f.payload["jti"].(string), "1")
				defer f.redis.Del("jwt:blacklist:" + f.payload["jti"].(string))
			case "missing floor", "corrupt floor":
				key := "auth:session:min_epoch:" + f.payload["user_id"].(string)
				if mode == "missing floor" {
					f.redis.Del(key)
				} else {
					f.redis.Set(key, "invalid")
				}
				defer f.redis.Set(key, "7")
				want = 503
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/spaces", strings.NewReader(`{"name":"n"}`))
			r.Header.Set("Authorization", "Bearer "+token)
			if mode == "duplicate bearer" {
				r.Header.Add("Authorization", "Bearer "+token)
			}
			if mode == "query token" {
				r.Header.Del("Authorization")
				r.URL.RawQuery = "access_token=" + token
			}
			w := httptest.NewRecorder()
			cfg := f.config(client)
			if mode == "unbound issuer" {
				cfg.tokenValidator = voicejwt.NewJWKSValidator(f.jwksURL, "", "voice-client")
			}
			if mode == "unbound audience" {
				cfg.tokenValidator = voicejwt.NewJWKSValidator(f.jwksURL, "voice-auth", "")
			}
			newGateway(cfg).ServeHTTP(w, r)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, client.calls)
		})
	}
}

func TestSpaceCreatorNoFallbackOrAmbiguousRetry(t *testing.T) {
	f := newCreatorFixture(t)
	for _, mode := range []string{"missing protected", "missing issuer", "static", "chained static", "no strict", "missing transcoder", "ambiguous"} {
		t.Run(mode, func(t *testing.T) {
			client := &creatorClient{call: func(context.Context, *spacev1.CreateSpaceRequest) (*spacev1.CreateSpaceResponse, error) {
				return nil, status.Error(codes.Unavailable, "ambiguous response")
			}}
			cfg := f.config(client)
			ordinary := &creatorClient{}
			cfg.transcoder.clients.space = ordinary
			proxyCalls := 0
			cfg.restUpstreams = map[string]http.Handler{"spaces": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxyCalls++ })}
			want := 503
			switch mode {
			case "missing protected":
				cfg.transcoder.clients.spaceLifecycle = nil
			case "missing issuer":
				cfg.transcoder.lifecycleIssuer = nil
			case "static", "chained static":
				token := signConfigJWT(t, "creator-key", f.key, f.payload)
				static := staticTokenValidator{token: {UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), AccountType: "regular", SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute)}}
				cfg.tokenValidator = static
				if mode == "chained static" {
					cfg.tokenValidator = chainedTokenValidator{static: static}
				}
				want = 401
			case "no strict":
				cfg.sessionEpochStrict = false
			case "missing transcoder":
				cfg.transcoder = nil
			}
			w := creatorRequest(t, newGateway(cfg), f, f.payload, `{"name":"n"}`)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, ordinary.calls)
			require.Zero(t, proxyCalls)
			if mode == "ambiguous" {
				require.Equal(t, 1, client.calls)
			} else {
				require.Zero(t, client.calls)
			}
		})
	}
}
