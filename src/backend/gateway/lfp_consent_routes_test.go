package main

import (
	"context"
	"crypto/rsa"
	"encoding/json"
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
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/pkg/principal"
)

type gatewayConsentClient struct {
	t                          *testing.T
	f                          *creatorFixture
	story, incarnation, author uuid.UUID
	calls                      int
	error                      error
	tokens                     []string
}

func (c *gatewayConsentClient) verify(ctx context.Context, r proto.Message, method string) {
	c.t.Helper()
	c.calls++
	md, ok := metadata.FromOutgoingContext(ctx)
	require.True(c.t, ok)
	require.Len(c.t, md, 2)
	require.Len(c.t, md.Get("authorization"), 1)
	require.Len(c.t, md.Get("x-request-id"), 1)
	h, e := principal.RequestHash(r)
	require.NoError(c.t, e)
	token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
	p, e := principal.VerifyDelegatedUser(ctx, token, principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "story", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: h, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &c.f.key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(_ context.Context, a string, epoch int64) error {
		require.Equal(c.t, c.f.payload["user_id"], a)
		require.EqualValues(c.t, 7, epoch)
		return nil
	}})
	require.NoError(c.t, e)
	require.Equal(c.t, c.f.payload["account_type"], p.AccountType)
	require.Equal(c.t, c.f.payload["profile_id"], p.ProfileID)
	c.tokens = append(c.tokens, token)
}
func (c *gatewayConsentClient) offer() *storyv1.LfpConsentOfferView {
	return &storyv1.LfpConsentOfferView{ProtocolVersion: 1, StoryId: c.story[:], StoryIncarnation: c.incarnation[:], AuthorProfileId: c.author[:], OfferRevision: 2, CriteriaSha256: append([]byte{1}, make([]byte, 31)...), OfferPayloadSha256: append([]byte{2}, make([]byte, 31)...), OfferExpiresAtUnixMillis: time.Now().Add(time.Minute).UnixMilli()}
}
func (c *gatewayConsentClient) PublishLfpConsentOffer(ctx context.Context, r *storyv1.PublishLfpConsentOfferRequest, _ ...grpc.CallOption) (*storyv1.PublishLfpConsentOfferResponse, error) {
	c.verify(ctx, r, storyv1.LfpConsentActionService_PublishLfpConsentOffer_FullMethodName)
	require.Equal(c.t, c.story[:], r.StoryId)
	require.Len(c.t, r.PublicationOperationId, 16)
	if c.error != nil {
		return nil, c.error
	}
	return &storyv1.PublishLfpConsentOfferResponse{Offer: c.offer()}, nil
}
func (c *gatewayConsentClient) ObserveLfpConsentOffer(ctx context.Context, r *storyv1.ObserveLfpConsentOfferRequest, _ ...grpc.CallOption) (*storyv1.ObserveLfpConsentOfferResponse, error) {
	c.verify(ctx, r, storyv1.LfpConsentActionService_ObserveLfpConsentOffer_FullMethodName)
	require.Equal(c.t, c.story[:], r.StoryId)
	return &storyv1.ObserveLfpConsentOfferResponse{Offer: c.offer()}, c.error
}
func (c *gatewayConsentClient) RespondWithLfpConsent(ctx context.Context, r *storyv1.RespondWithLfpConsentRequest, _ ...grpc.CallOption) (*storyv1.RespondWithLfpConsentResponse, error) {
	c.verify(ctx, r, storyv1.LfpConsentActionService_RespondWithLfpConsent_FullMethodName)
	require.Equal(c.t, c.story[:], r.Offer.StoryId)
	require.Len(c.t, r.ResponseOperationId, 16)
	return &storyv1.RespondWithLfpConsentResponse{}, c.error
}

func gatewayConsentRequest(t *testing.T, h http.Handler, f *creatorFixture, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, f.payload))
	r.Header.Set("X-Voice-User-Id", uuid.NewString())
	r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	r.Header.Set("X-Voice-Account-Type", "regular")
	r.Header.Set("X-Voice-Session-Epoch", "99")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestStoryConsentHTTPRealJWTAndRedis(t *testing.T) {
	for _, kind := range []string{"guest", "regular"} {
		t.Run(kind, func(t *testing.T) {
			f := newCreatorFixture(t)
			f.payload["account_type"] = kind
			client := &gatewayConsentClient{t: t, f: f, story: uuid.New(), incarnation: uuid.New(), author: uuid.New()}
			cfg := f.config(nil)
			cfg.transcoder.clients.space = nil
			cfg.transcoder.clients.story = nil
			cfg.transcoder.clients.storyConsent = client
			h := newGateway(cfg)
			path := "/api/v1/stories/" + client.story.String()
			op := uuid.NewString()
			body := `{"operation_id":"` + op + `"}`
			for range 2 {
				w := gatewayConsentRequest(t, h, f, http.MethodPost, path+"/lfp-consent-offer", body)
				require.Equal(t, 200, w.Code, w.Body.String())
				require.NotContains(t, w.Body.String(), "account_id")
				require.NotContains(t, w.Body.String(), "session_epoch")
			}
			require.NotEqual(t, client.tokens[0], client.tokens[1], "stable domain operation uses fresh delegated credential each attempt")
			w := gatewayConsentRequest(t, h, f, http.MethodGet, path+"/lfp-consent-offer", "")
			require.Equal(t, 200, w.Code, w.Body.String())
			var view struct {
				Offer publicLfpOffer `json:"offer"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &view))
			for _, responseType := range []string{"join", "invite"} {
				b, e := json.Marshal(struct {
					OperationID string         `json:"operation_id"`
					Offer       publicLfpOffer `json:"offer"`
					Type        string         `json:"response_type"`
				}{uuid.NewString(), view.Offer, responseType})
				require.NoError(t, e)
				w = gatewayConsentRequest(t, h, f, http.MethodPost, path+"/lfp-consent-response", string(b))
				require.Equal(t, 204, w.Code, w.Body.String())
				require.Empty(t, w.Body.String())
			}
			calls := client.calls
			w = gatewayConsentRequest(t, h, f, http.MethodPost, path+"/lfp-consent-offer", `{"operation_id":"`+op+`","account_id":"forged"}`)
			require.Equal(t, 400, w.Code)
			require.Equal(t, calls, client.calls)
			client.error = status.Error(codes.Unavailable, "lost ACK")
			w = gatewayConsentRequest(t, h, f, http.MethodPost, path+"/lfp-consent-offer", body)
			require.Equal(t, 503, w.Code)
		})
	}
}

func TestStoryConsentHTTPDeniesUnqualifiedAuthority(t *testing.T) {
	for _, mode := range []string{"static", "missing type", "SDK", "floor revoked", "blacklist revoked", "missing issuer", "missing strict", "missing client"} {
		t.Run(mode, func(t *testing.T) {
			f := newCreatorFixture(t)
			client := &gatewayConsentClient{t: t, f: f, story: uuid.New(), incarnation: uuid.New(), author: uuid.New()}
			cfg := f.config(nil)
			cfg.transcoder.clients.storyConsent = client
			want := 401
			switch mode {
			case "static":
				token := signConfigJWT(t, "creator-key", f.key, f.payload)
				cfg.tokenValidator = staticTokenValidator{token: {UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), AccountType: "regular", AuthenticatedAccountType: "regular", SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute)}}
			case "missing type":
				delete(f.payload, "account_type")
				want = 403
			case "SDK":
				f.payload["account_type"] = "sdk-account"
				want = 403
			case "floor revoked":
				f.redis.Set("auth:session:min_epoch:"+f.payload["user_id"].(string), "8")
			case "blacklist revoked":
				f.redis.Set("jwt:blacklist:"+f.payload["jti"].(string), "1")
			case "missing issuer":
				cfg.tokenValidator = nil // A non-JWKS validator cannot mint protected human actions.
			case "missing strict":
				cfg.sessionEpochStrict = false
				want = 503
			case "missing client":
				cfg.transcoder.clients.storyConsent = nil
				want = 503
			}
			w := gatewayConsentRequest(t, newGateway(cfg), f, http.MethodPost, "/api/v1/stories/"+client.story.String()+"/lfp-consent-offer", `{"operation_id":"`+uuid.NewString()+`"}`)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, client.calls)
		})
	}
}

func TestStoryConsentTLSConfigurationIsAllOrNone(t *testing.T) {
	for _, name := range []string{"GATEWAY_STORY_CONSENT_GRPC_ADDR", "GATEWAY_STORY_CONSENT_TLS_CA_FILE", "GATEWAY_STORY_CONSENT_TLS_SERVER_NAME", "GATEWAY_STORY_CONSENT_CLIENT_CERT_FILE", "GATEWAY_STORY_CONSENT_CLIENT_KEY_FILE"} {
		t.Setenv(name, "")
	}
	_, enabled, e := storyConsentClientConfigFromEnv()
	require.NoError(t, e)
	require.False(t, enabled)
	t.Setenv("GATEWAY_STORY_CONSENT_GRPC_ADDR", "story:9092")
	_, enabled, e = storyConsentClientConfigFromEnv()
	require.Error(t, e)
	require.True(t, enabled)
}

func TestStoryConsentWithoutTranscoderNeverFallsBackToREST(t *testing.T) {
	for _, kind := range []string{"guest", "regular"} {
		for _, route := range []struct{ method, suffix string }{
			{http.MethodPost, "/lfp-consent-offer"},
			{http.MethodGet, "/lfp-consent-offer"},
			{http.MethodPost, "/lfp-consent-response"},
		} {
			t.Run(kind+route.method+route.suffix, func(t *testing.T) {
				f := newCreatorFixture(t)
				f.payload["account_type"] = kind
				cfg := f.config(nil)
				cfg.transcoder = nil
				proxyCalls := 0
				cfg.restUpstreams = map[string]http.Handler{"stories": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					proxyCalls++
					w.WriteHeader(http.StatusNoContent)
				})}
				path := "/api/v1/stories/" + uuid.NewString() + route.suffix
				w := gatewayConsentRequest(t, newGateway(cfg), f, route.method, path, `{}`)
				require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
				require.Zero(t, proxyCalls)
			})
		}
	}
}
