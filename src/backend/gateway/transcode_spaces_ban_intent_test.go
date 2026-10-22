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
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type banIntentClient struct {
	spacev1.SpaceServiceClient
	calls int
	call  func(context.Context, proto.Message, string) error
}

func (c *banIntentClient) BanMember(ctx context.Context, req *spacev1.BanMemberRequest, _ ...grpc.CallOption) (*spacev1.BanMemberResponse, error) {
	c.calls++
	if c.call != nil {
		if err := c.call(ctx, req, spacev1.SpaceService_BanMember_FullMethodName); err != nil {
			return nil, err
		}
	}
	return &spacev1.BanMemberResponse{}, nil
}
func (c *banIntentClient) UnbanMember(ctx context.Context, req *spacev1.UnbanMemberRequest, _ ...grpc.CallOption) (*spacev1.UnbanMemberResponse, error) {
	c.calls++
	if c.call != nil {
		if err := c.call(ctx, req, spacev1.SpaceService_UnbanMember_FullMethodName); err != nil {
			return nil, err
		}
	}
	return &spacev1.UnbanMemberResponse{}, nil
}

func banIntentRequest(t *testing.T, h http.Handler, f *creatorFixture, method, path, body string, headers []string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, f.payload))
	r.Header.Set("X-Voice-User-Id", uuid.NewString())
	r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	r.Header.Set("X-Voice-Session-Epoch", "99")
	r.Header.Set("X-Request-Id", "untrusted-client-request")
	if headers != nil {
		r.Header["Idempotency-Key"] = headers
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSpaceBanIntentProtectedStableBodyFreshTransport(t *testing.T) {
	f := newCreatorFixture(t)
	space, account, op := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		t.Run(method, func(t *testing.T) {
			path := "/api/v1/spaces/" + space + "/bans"
			body := `{"operation_id":"` + op + `","account_id":"` + account + `","space_id":"` + space + `"}`
			if method == http.MethodDelete {
				path += "/" + account
			} else {
				body = strings.TrimSuffix(body, "}") + `,"reason":"spam","profile_id":"` + uuid.NewString() + `"}`
			}
			protected, ordinary := &banIntentClient{}, &banIntentClient{}
			var ids, jtis []string
			var first []byte
			protected.call = func(ctx context.Context, req proto.Message, fullMethod string) error {
				field := req.ProtoReflect().Descriptor().Fields().ByName("operation_id")
				require.NotNil(t, field)
				require.Equal(t, op, req.ProtoReflect().Get(field).String())
				require.Equal(t, space, req.ProtoReflect().Get(req.ProtoReflect().Descriptor().Fields().ByName("space_id")).String())
				require.Equal(t, account, req.ProtoReflect().Get(req.ProtoReflect().Descriptor().Fields().ByName("account_id")).String())
				bytes, err := proto.MarshalOptions{Deterministic: true}.Marshal(req)
				require.NoError(t, err)
				if first == nil {
					first = bytes
				} else {
					require.Equal(t, first, bytes, "retry must retain the exact logical body")
				}
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				require.Len(t, md, 2, "raw identity or operation metadata must not confer authority")
				require.Len(t, md.Get("x-request-id"), 1)
				id := md.Get("x-request-id")[0]
				require.True(t, canonicalLifecycleUUID(id))
				require.NotEqual(t, op, id)
				token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				cfg := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: fullMethod, ExpectedRequestID: id, ExpectedRequestHash: hash,
					KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
					ReplayGuard: func(_ context.Context, _ string, jti string, _ time.Time) error { jtis = append(jtis, jti); return nil },
					SessionEpochChecker: func(_ context.Context, a string, e int64) error {
						require.Equal(t, f.payload["user_id"], a)
						require.EqualValues(t, 7, e)
						return nil
					}}
				p, err := principal.VerifyDelegatedUser(ctx, token, cfg)
				require.NoError(t, err)
				require.Equal(t, f.payload["profile_id"], p.ProfileID)
				changed := proto.Clone(req)
				changed.ProtoReflect().Set(field, protoreflect.ValueOfString(uuid.NewString()))
				cfg.ExpectedRequestHash, err = principal.RequestHash(changed)
				require.NoError(t, err)
				_, err = principal.VerifyDelegatedUser(ctx, token, cfg)
				require.Error(t, err, "changing logical intent invalidates the signed full body")
				ids = append(ids, id)
				return nil
			}
			cfg := f.config(protected)
			cfg.transcoder.clients.space = ordinary
			for i := 0; i < 2; i++ {
				if i == 1 {
					cfg.transcoder.clients.space = nil
				}
				w := banIntentRequest(t, newGateway(cfg), f, method, path, body, []string{op})
				require.Equal(t, 204, w.Code, w.Body.String())
				require.Empty(t, w.Body.String())
			}
			require.Equal(t, 2, protected.calls)
			require.Zero(t, ordinary.calls)
			require.NotEqual(t, ids[0], ids[1])
			require.Len(t, jtis, 2)
			require.NotEqual(t, jtis[0], jtis[1])
		})
	}
}

func TestSpaceBanIntentRejectsInvalidBeforeDispatch(t *testing.T) {
	f := newCreatorFixture(t)
	space, account, op := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		for _, tc := range []struct {
			name, body, suffix string
			headers            []string
		}{
			{"empty body", "", "", nil}, {"missing operation", `{}`, "", nil},
			{"header only", `{}`, "", []string{op}},
			{"nil operation", `{"operation_id":"` + uuid.Nil.String() + `"}`, "", nil},
			{"uppercase operation", `{"operation_id":"` + strings.ToUpper(op) + `"}`, "", nil},
			{"padded operation", `{"operation_id":" ` + op + `"}`, "", nil},
			{"unknown", `{"operation_id":"` + op + `","actor_profile_id":"` + uuid.NewString() + `"}`, "", nil},
			{"duplicate alias", `{"operation_id":"` + op + `","operationId":"` + op + `"}`, "", nil},
			{"foreign space", `{"operation_id":"` + op + `","space_id":"` + uuid.NewString() + `"}`, "", nil},
			{"empty body space", `{"operation_id":"` + op + `","space_id":""}`, "", nil},
			{"foreign account", `{"operation_id":"` + op + `","account_id":"` + uuid.NewString() + `"}`, "", nil},
			{"empty body account", `{"operation_id":"` + op + `","account_id":""}`, "", nil},
			{"query", `{"operation_id":"` + op + `"}`, "?operation_id=" + op, nil},
			{"empty header", `{"operation_id":"` + op + `"}`, "", []string{""}},
			{"uppercase header", `{"operation_id":"` + op + `"}`, "", []string{strings.ToUpper(op)}},
			{"mismatch header", `{"operation_id":"` + op + `"}`, "", []string{uuid.NewString()}},
			{"duplicate header", `{"operation_id":"` + op + `"}`, "", []string{op, op}},
			{"oversized", `{"operation_id":"` + op + `","reason":"` + strings.Repeat("x", 64<<10) + `"}`, "", nil},
		} {
			if method == http.MethodPost && tc.name == "foreign account" {
				continue // POST has no account path component; its body selects the target.
			}
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				client, ordinary := &banIntentClient{}, &banIntentClient{}
				cfg := f.config(client)
				cfg.transcoder.clients.space = ordinary
				path := "/api/v1/spaces/" + space + "/bans"
				body := tc.body
				if method == http.MethodDelete {
					path += "/" + account
				} else if body != "" && !strings.Contains(body, `"account_id"`) {
					body = strings.TrimSuffix(body, "}") + `,"account_id":"` + account + `"}`
				}
				w := banIntentRequest(t, newGateway(cfg), f, method, path+tc.suffix, body, tc.headers)
				require.Equal(t, 400, w.Code, w.Body.String())
				require.Zero(t, client.calls)
				require.Zero(t, ordinary.calls)
			})
		}
	}
}

func TestSpaceBanIntentNoFallbackOrAutomaticRetry(t *testing.T) {
	f := newCreatorFixture(t)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		for _, mode := range []string{"missing signer", "missing protected", "missing transcoder", "static", "no strict", "ambiguous"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				client := &banIntentClient{call: func(context.Context, proto.Message, string) error {
					return status.Error(codes.Unavailable, "ambiguous")
				}}
				ordinary := &banIntentClient{}
				cfg := f.config(client)
				cfg.transcoder.clients.space = ordinary
				proxy := 0
				cfg.restUpstreams = map[string]http.Handler{"spaces": http.HandlerFunc(func(http.ResponseWriter, *http.Request) { proxy++ })}
				want := 503
				switch mode {
				case "missing signer":
					cfg.transcoder.lifecycleIssuer = nil
				case "missing protected":
					cfg.transcoder.clients.spaceLifecycle = nil
				case "missing transcoder":
					cfg.transcoder = nil
				case "no strict":
					cfg.sessionEpochStrict = false
				case "static":
					token := signConfigJWT(t, "creator-key", f.key, f.payload)
					cfg.tokenValidator = staticTokenValidator{token: {UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), SessionEpoch: 7, AccountType: "regular", AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}
					want = 401
				}
				space, account, op := uuid.NewString(), uuid.NewString(), uuid.NewString()
				path := "/api/v1/spaces/" + space + "/bans"
				if method == http.MethodDelete {
					path += "/" + account
				}
				w := banIntentRequest(t, newGateway(cfg), f, method, path, `{"account_id":"`+account+`","operation_id":"`+op+`"}`, nil)
				require.Equal(t, want, w.Code, w.Body.String())
				require.Zero(t, ordinary.calls)
				require.Zero(t, proxy)
				if mode == "ambiguous" {
					require.Equal(t, 1, client.calls)
				} else {
					require.Zero(t, client.calls)
				}
			})
		}
	}
}

func TestSpaceBanIntentPathAndUUIDBinding(t *testing.T) {
	f := newCreatorFixture(t)
	space, account, op := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name, method, path, body string
		want                     int
	}{
		{"camel POST", http.MethodPost, space + "/bans", `{"accountId":"` + account + `","operationId":"` + op + `"}`, 204},
		{"camel DELETE", http.MethodDelete, space + "/bans/" + account, `{"accountId":"` + account + `","spaceId":"` + space + `","operationId":"` + op + `"}`, 204},
		{"bad space", http.MethodPost, "invalid/bans", `{"account_id":"` + account + `","operation_id":"` + op + `"}`, 400},
		{"bad account path", http.MethodDelete, space + "/bans/invalid", `{"operation_id":"` + op + `"}`, 400},
		{"nested account path", http.MethodDelete, space + "/bans/" + account + "/extra", `{"operation_id":"` + op + `"}`, 400},
		{"missing account path", http.MethodDelete, space + "/bans", `{"operation_id":"` + op + `"}`, 400},
		{"POST account path", http.MethodPost, space + "/bans/" + account, `{"account_id":"` + account + `","operation_id":"` + op + `"}`, 400},
		{"missing account", http.MethodPost, space + "/bans", `{"operation_id":"` + op + `"}`, 400},
		{"empty profile", http.MethodPost, space + "/bans", `{"account_id":"` + account + `","operation_id":"` + op + `","profile_id":""}`, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, ordinary := &banIntentClient{}, &banIntentClient{}
			cfg := f.config(client)
			cfg.transcoder.clients.space = ordinary
			w := banIntentRequest(t, newGateway(cfg), f, tc.method, "/api/v1/spaces/"+tc.path, tc.body, nil)
			require.Equal(t, tc.want, w.Code, w.Body.String())
			require.Zero(t, ordinary.calls)
			if tc.want == 204 {
				require.Equal(t, 1, client.calls)
			} else {
				require.Zero(t, client.calls)
			}
		})
	}
}

func TestSpaceBanIntentRequiresCurrentDelegationSession(t *testing.T) {
	f := newCreatorFixture(t)
	original := cloneConfigJWTPayload(f.payload)
	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		for _, mode := range []string{"missing jti", "stale epoch", "missing floor"} {
			t.Run(method+"/"+mode, func(t *testing.T) {
				f.payload = cloneConfigJWTPayload(original)
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
				}
				client, ordinary := &banIntentClient{}, &banIntentClient{}
				cfg := f.config(client)
				cfg.transcoder.clients.space = ordinary
				space, account, op := uuid.NewString(), uuid.NewString(), uuid.NewString()
				path := "/api/v1/spaces/" + space + "/bans"
				if method == http.MethodDelete {
					path += "/" + account
				}
				w := banIntentRequest(t, newGateway(cfg), f, method, path, `{"account_id":"`+account+`","operation_id":"`+op+`"}`, nil)
				require.Equal(t, want, w.Code, w.Body.String())
				require.Zero(t, client.calls)
				require.Zero(t, ordinary.calls)
			})
		}
	}
}

func TestSpaceBanIntentErrorMapping(t *testing.T) {
	f := newCreatorFixture(t)
	for _, tc := range []struct {
		name           string
		code           codes.Code
		domain, reason string
		want           int
	}{
		{"in progress", codes.FailedPrecondition, "voice.space.moderation.v1", "INTENT_IN_PROGRESS", 409},
		{"owner", codes.FailedPrecondition, "", "", 412}, {"expired retained", codes.FailedPrecondition, "", "", 412},
		{"wrong domain", codes.FailedPrecondition, "other", "INTENT_IN_PROGRESS", 412},
		{"wrong reason", codes.FailedPrecondition, "voice.space.moderation.v1", "OTHER", 412},
		{"wrong code", codes.PermissionDenied, "voice.space.moderation.v1", "INTENT_IN_PROGRESS", 403},
		{"changed body", codes.AlreadyExists, "", "", 409}, {"unavailable", codes.Unavailable, "", "", 503},
		{"transport timeout", codes.DeadlineExceeded, "", "", 503},
	} {
		for _, method := range []string{http.MethodPost, http.MethodDelete} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				st := status.New(tc.code, "INTENT_IN_PROGRESS private backend detail")
				if tc.domain != "" {
					var err error
					st, err = st.WithDetails(&errdetails.ErrorInfo{Domain: tc.domain, Reason: tc.reason})
					require.NoError(t, err)
				}
				client := &banIntentClient{call: func(context.Context, proto.Message, string) error { return st.Err() }}
				cfg := f.config(client)
				cfg.transcoder.clients.space = &banIntentClient{}
				space, account, op := uuid.NewString(), uuid.NewString(), uuid.NewString()
				path := "/api/v1/spaces/" + space + "/bans"
				if method == http.MethodDelete {
					path += "/" + account
				}
				w := banIntentRequest(t, newGateway(cfg), f, method, path, `{"account_id":"`+account+`","operation_id":"`+op+`"}`, nil)
				require.Equal(t, tc.want, w.Code, w.Body.String())
				require.Equal(t, 1, client.calls)
				require.NotContains(t, w.Body.String(), "private backend detail")
				var response map[string]string
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				expectedCode := tc.code
				if expectedCode == codes.DeadlineExceeded {
					expectedCode = codes.Unavailable
				}
				require.Equal(t, grpcCodeToErrorCode(expectedCode), response["error_code"])
			})
		}
	}
	// Typed moderation details cannot change any unrelated method's generic mapper.
	st, err := status.New(codes.FailedPrecondition, "owner").WithDetails(&errdetails.ErrorInfo{Domain: "voice.space.moderation.v1", Reason: "INTENT_IN_PROGRESS"})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	writeGRPCError(w, st.Err())
	require.Equal(t, 412, w.Code)
	w = httptest.NewRecorder()
	writeGRPCError(w, status.Error(codes.DeadlineExceeded, "unrelated timeout"))
	require.Equal(t, 500, w.Code, "moderation normalization must not alter the global mapper")
}
