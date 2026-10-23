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
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type joinClient struct {
	spacev1.SpaceServiceClient
	calls int
	call  func(context.Context, proto.Message, string) error
}

func (c *joinClient) JoinSpace(ctx context.Context, req *spacev1.JoinSpaceRequest, _ ...grpc.CallOption) (*spacev1.JoinSpaceResponse, error) {
	c.calls++
	if c.call != nil {
		if err := c.call(ctx, req, spacev1.SpaceService_JoinSpace_FullMethodName); err != nil {
			return nil, err
		}
	}
	return &spacev1.JoinSpaceResponse{SpaceMembership: &spacev1.SpaceMembership{SpaceId: req.SpaceId}}, nil
}
func (c *joinClient) JoinByInvite(ctx context.Context, req *spacev1.JoinByInviteRequest, _ ...grpc.CallOption) (*spacev1.JoinByInviteResponse, error) {
	c.calls++
	if c.call != nil {
		if err := c.call(ctx, req, spacev1.SpaceService_JoinByInvite_FullMethodName); err != nil {
			return nil, err
		}
	}
	return &spacev1.JoinByInviteResponse{SpaceMembership: &spacev1.SpaceMembership{SpaceId: "joined"}}, nil
}
func joinRequest(t *testing.T, h http.Handler, f *creatorFixture, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, f.payload))
	r.Header.Set("X-Voice-User-Id", uuid.NewString())
	r.Header.Set("X-Voice-Account-Type", "regular")
	r.Header.Set("X-Request-Id", "spoof")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func TestSpaceJoinProtectedDispatch(t *testing.T) {
	f := newCreatorFixture(t)
	for _, kind := range []string{"regular", "guest"} {
		f.payload["account_type"] = kind
		for _, path := range []string{"/api/v1/spaces/" + uuid.NewString() + "/join", "/api/v1/invites/exact-code/join"} {
			t.Run(kind+path, func(t *testing.T) {
				protected := &joinClient{}
				ordinary := &joinClient{}
				var ids, tokens []string
				protected.call = func(ctx context.Context, req proto.Message, method string) error {
					md, ok := metadata.FromOutgoingContext(ctx)
					require.True(t, ok)
					require.Len(t, md, 2)
					require.Len(t, md.Get("authorization"), 1)
					require.Len(t, md.Get("x-request-id"), 1)
					id := md.Get("x-request-id")[0]
					require.True(t, canonicalLifecycleUUID(id))
					token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
					hash, err := principal.RequestHash(req)
					require.NoError(t, err)
					cfg := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
						require.Equal(t, f.payload["user_id"], account)
						require.EqualValues(t, 7, epoch)
						return nil
					}}
					p, err := principal.VerifyDelegatedUser(ctx, token, cfg)
					require.NoError(t, err)
					require.Equal(t, f.payload["profile_id"], p.ProfileID)
					require.Equal(t, kind, p.AccountType)
					require.LessOrEqual(t, p.ExpiresAt.Unix(), f.payload["exp"].(int64))
					bad := cfg
					bad.ExpectedRequestHash = "changed"
					_, err = principal.VerifyDelegatedUser(ctx, token, bad)
					require.Error(t, err)
					ids = append(ids, id)
					tokens = append(tokens, token)
					return nil
				}
				cfg := f.config(protected)
				cfg.transcoder.clients.space = ordinary
				for i := 0; i < 2; i++ {
					if i == 1 {
						cfg.transcoder.clients.space = nil
					}
					w := joinRequest(t, newGateway(cfg), f, path, "")
					require.Equal(t, 200, w.Code, w.Body.String())
				}
				require.Equal(t, 2, protected.calls)
				require.Zero(t, ordinary.calls)
				require.NotEqual(t, ids[0], ids[1])
				require.NotEqual(t, tokens[0], tokens[1])
			})
		}
	}
}

func TestSpaceJoinAuthenticationFailures(t *testing.T) {
	f := newCreatorFixture(t)
	original := cloneConfigJWTPayload(f.payload)
	for _, path := range []string{"/api/v1/spaces/" + uuid.NewString() + "/join", "/api/v1/invites/code/join", "/api/v1/spaces/" + uuid.NewString() + "/voice-rooms"} {
		for _, mode := range []string{"missing jti", "stale epoch", "missing floor", "revoked", "missing profile", "expired", "wrong audience", "padded type"} {
			t.Run(mode+path, func(t *testing.T) {
				f.payload = cloneConfigJWTPayload(original)
				client := &joinClient{}
				want := 401
				switch mode {
				case "missing jti":
					delete(f.payload, "jti")
				case "stale epoch":
					f.payload["session_epoch"] = 6
				case "missing floor":
					key := "auth:session:min_epoch:" + f.payload["user_id"].(string)
					f.redis.Del(key)
					defer f.redis.Set(key, "7")
					want = 503
				case "revoked":
					key := "jwt:blacklist:" + f.payload["jti"].(string)
					f.redis.Set(key, "1")
					defer f.redis.Del(key)
				case "missing profile":
					delete(f.payload, "profile_id")
				case "expired":
					f.payload["exp"] = time.Now().Add(-time.Minute).Unix()
				case "wrong audience":
					f.payload["aud"] = "other"
				case "padded type":
					f.payload["account_type"] = " regular "
					want = 403
				}
				body := ""
				if strings.HasSuffix(path, "/voice-rooms") {
					body = `{"name":"Voice"}`
				}
				w := joinRequest(t, newGateway(f.config(client)), f, path, body)
				require.Equal(t, want, w.Code, w.Body.String())
				require.Zero(t, client.calls)
			})
		}
	}
}

func TestSpaceJoinOperationBindingFailsClosed(t *testing.T) {
	for _, values := range [][]string{{""}, {"bad"}, {uuid.Nil.String()}, {" " + uuid.NewString()}, {uuid.NewString(), uuid.NewString()}} {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/invites/code/join", nil)
		r.Header["Idempotency-Key"] = values
		req := &spacev1.JoinByInviteRequest{Code: "exact-code"}
		require.Equal(t, codes.InvalidArgument, status.Code(bindJoinOperation(r, req)))
		require.Equal(t, "exact-code", req.Code)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/invites/code/join", nil)
	r.Header.Set("Idempotency-Key", uuid.NewString())
	// An older/incorrect wire schema must never silently discard a valid key.
	require.Equal(t, codes.Unavailable, status.Code(bindJoinOperation(r, &spacev1.GetInviteRequest{Code: "code"})))
}

func TestSpaceJoinOperationKeyIsSigned(t *testing.T) {
	f := newCreatorFixture(t)
	for _, path := range []string{"/api/v1/spaces/" + uuid.NewString() + "/join", "/api/v1/invites/exact-code/join"} {
		t.Run(path, func(t *testing.T) {
			operation := uuid.NewString()
			client := &joinClient{call: func(ctx context.Context, req proto.Message, method string) error {
				field := req.ProtoReflect().Descriptor().Fields().ByName("operation_id")
				require.NotNil(t, field)
				require.True(t, req.ProtoReflect().Has(field))
				require.Equal(t, operation, req.ProtoReflect().Get(field).String())
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Len(t, md, 2)
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				cfg := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
					KeyResolver:         func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
					ReplayGuard:         func(context.Context, string, string, time.Time) error { return nil },
					SessionEpochChecker: func(context.Context, string, int64) error { return nil }}
				token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
				_, err = principal.VerifyDelegatedUser(ctx, token, cfg)
				require.NoError(t, err)
				changed := proto.Clone(req)
				changed.ProtoReflect().Clear(field)
				cfg.ExpectedRequestHash, err = principal.RequestHash(changed)
				require.NoError(t, err)
				_, err = principal.VerifyDelegatedUser(ctx, token, cfg)
				require.Error(t, err, "removing operation presence must invalidate signed body")
				return nil
			}}
			r := httptest.NewRequest(http.MethodPost, path, nil)
			r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, f.payload))
			r.Header.Set("Idempotency-Key", strings.ToUpper(operation))
			w := httptest.NewRecorder()
			newGateway(f.config(client)).ServeHTTP(w, r)
			require.Equal(t, 200, w.Code, w.Body.String())
			require.Equal(t, 1, client.calls)
		})
	}
}
func TestSpaceJoinFailsClosed(t *testing.T) {
	f := newCreatorFixture(t)
	for _, path := range []string{"/api/v1/spaces/" + uuid.NewString() + "/join", "/api/v1/invites/code/join"} {
		for _, mode := range []string{"body", "missing signer", "missing protected", "static", "no strict", "missing transcoder", "ambiguous", "missing type", "unknown type"} {
			t.Run(mode+path, func(t *testing.T) {
				f.payload["account_type"] = "regular"
				protected := &joinClient{call: func(context.Context, proto.Message, string) error {
					return status.Error(codes.Unavailable, "ambiguous")
				}}
				ordinary := &joinClient{}
				cfg := f.config(protected)
				cfg.transcoder.clients.space = ordinary
				proxy := 0
				cfg.restUpstreams = map[string]http.Handler{"spaces": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxy++ }), "invites": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxy++ })}
				want := 503
				body := ""
				switch mode {
				case "body":
					body = `{"proof":"hidden"}`
					want = 400
				case "missing signer":
					cfg.transcoder.lifecycleIssuer = nil
				case "missing protected":
					cfg.transcoder.clients.spaceLifecycle = nil
				case "static":
					token := signConfigJWT(t, "creator-key", f.key, f.payload)
					cfg.tokenValidator = staticTokenValidator{token: {UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), SessionEpoch: 7, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}
					want = 401
				case "no strict":
					cfg.sessionEpochStrict = false
				case "missing transcoder":
					cfg.transcoder = nil
				case "missing type":
					delete(f.payload, "account_type")
					want = 403
				case "unknown type":
					f.payload["account_type"] = "sdk-account"
					want = 403
				}
				w := joinRequest(t, newGateway(cfg), f, path, body)
				require.Equal(t, want, w.Code, w.Body.String())
				require.Zero(t, ordinary.calls)
				require.Zero(t, proxy)
				if mode == "ambiguous" {
					require.Equal(t, 1, protected.calls)
				} else {
					require.Zero(t, protected.calls)
				}
			})
		}
	}
}
