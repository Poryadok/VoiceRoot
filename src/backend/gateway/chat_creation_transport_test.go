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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/principal"
)

func standaloneCreateRequest(t *testing.T, cfg gatewayConfig, f *creatorFixture, payload map[string]any, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/chats", strings.NewReader(body))
	r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, payload))
	r.Header.Set("X-Voice-User-Id", uuid.NewString())
	r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
	r.Header.Set("X-Voice-Account-Type", "regular")
	r.Header.Set("X-Voice-Session-Epoch", "99")
	r.Header.Set("X-Request-Id", "untrusted-transport-id")
	w := httptest.NewRecorder()
	newGateway(cfg).ServeHTTP(w, r)
	return w
}

func TestStandaloneChatCreationProtectedDispatch(t *testing.T) {
	f := newCreatorFixture(t)
	for _, kind := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL} {
		for _, keyed := range []bool{false, true} {
			t.Run(kind.String()+map[bool]string{false: "/unkeyed", true: "/keyed"}[keyed], func(t *testing.T) {
				want := &chatv1.CreateChatRequest{Type: kind, Name: proto.String("Friday squad")}
				body := `{"type":"` + kind.String() + `","name":"Friday squad"`
				if keyed {
					want.RequestId = uuid.NewString()
					body += `,"request_id":"` + want.RequestId + `"`
				}
				body += `}`
				ordinaryCalls, protectedCalls := 0, 0
				cfg := f.config(nil)
				cfg.transcoder.clients.chat = compositeChatClient{create: func(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
					ordinaryCalls++
					return nil, status.Error(codes.PermissionDenied, "protected Chat creation listener required")
				}}
				var ids, tokens []string
				cfg.transcoder.clients.chatAudit = compositeChatClient{create: func(ctx context.Context, req *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
					protectedCalls++
					require.True(t, proto.Equal(want, req), "business body must remain unchanged")
					md, ok := metadata.FromOutgoingContext(ctx)
					require.True(t, ok)
					require.Len(t, md, 2)
					require.Len(t, md.Get("authorization"), 1)
					require.Len(t, md.Get("x-request-id"), 1)
					id := md.Get("x-request-id")[0]
					require.True(t, canonicalLifecycleUUID(id))
					if keyed {
						require.Equal(t, want.RequestId, id)
					}
					token := strings.TrimPrefix(md.Get("authorization")[0], "Bearer ")
					hash, err := principal.RequestHash(want)
					require.NoError(t, err)
					verify := principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "chat", ExpectedRPC: chatv1.ChatService_CreateChat_FullMethodName, ExpectedRequestID: id, ExpectedRequestHash: hash,
						KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
						SessionEpochChecker: func(_ context.Context, account string, epoch int64) error {
							require.Equal(t, f.payload["user_id"], account)
							require.EqualValues(t, 7, epoch)
							return nil
						}}
					actor, err := principal.VerifyDelegatedUser(ctx, token, verify)
					require.NoError(t, err)
					require.Equal(t, f.payload["profile_id"], actor.ProfileID)
					require.Equal(t, "regular", actor.AccountType)
					for _, mutate := range []func(*principal.VerifyConfig){
						func(c *principal.VerifyConfig) { c.ExpectedAudience = "space" },
						func(c *principal.VerifyConfig) { c.ExpectedRPC = chatv1.ChatService_UpdateChat_FullMethodName },
						func(c *principal.VerifyConfig) { c.ExpectedRequestID = uuid.NewString() },
						func(c *principal.VerifyConfig) {
							changed := proto.Clone(want).(*chatv1.CreateChatRequest)
							changed.Name = proto.String(changed.GetName() + "tampered")
							c.ExpectedRequestHash, _ = principal.RequestHash(changed)
						},
					} {
						bad := verify
						mutate(&bad)
						_, err := principal.VerifyDelegatedUser(ctx, token, bad)
						require.Error(t, err)
					}
					ids, tokens = append(ids, id), append(tokens, token)
					return &chatv1.CreateChatResponse{Chat: &chatv1.Chat{Id: uuid.NewString(), Type: req.Type, Name: req.Name}}, nil
				}}
				for i := 0; i < 2; i++ {
					w := standaloneCreateRequest(t, cfg, f, f.payload, body)
					require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				}
				require.Zero(t, ordinaryCalls)
				require.Equal(t, 2, protectedCalls)
				require.NotEqual(t, tokens[0], tokens[1])
				if !keyed {
					require.NotEqual(t, ids[0], ids[1], "independent unkeyed retries have distinct transport IDs")
				}
			})
		}
	}
}

func TestStandaloneChatCreationStrictAuthentication(t *testing.T) {
	f := newCreatorFixture(t)
	for _, scenario := range []string{"missing type", "padded type", "missing jti", "missing profile", "nil account", "stale epoch", "wrong issuer", "wrong audience", "unbound issuer", "unbound audience", "static", "chained static", "compat epoch", "query token", "duplicate bearer", "raw headers"} {
		t.Run(scenario, func(t *testing.T) {
			payload := cloneConfigJWTPayload(f.payload)
			switch scenario {
			case "missing type":
				delete(payload, "account_type")
			case "padded type":
				payload["account_type"] = " regular "
			case "missing jti":
				delete(payload, "jti")
			case "missing profile":
				delete(payload, "profile_id")
			case "nil account":
				payload["user_id"] = uuid.Nil.String()
			case "stale epoch":
				payload["session_epoch"] = 6
			case "wrong issuer":
				payload["iss"] = "untrusted"
			case "wrong audience":
				payload["aud"] = "untrusted"
			}
			calls := 0
			client := compositeChatClient{create: func(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
				calls++
				return nil, status.Error(codes.Unavailable, "unexpected dispatch")
			}}
			cfg := f.config(nil)
			cfg.transcoder.clients.chat, cfg.transcoder.clients.chatAudit = client, client
			token := signConfigJWT(t, "creator-key", f.key, payload)
			want := http.StatusUnauthorized
			switch scenario {
			case "static", "chained static":
				static := staticTokenValidator{token: {UserID: f.payload["user_id"].(string), ProfileID: f.payload["profile_id"].(string), SessionEpoch: 7, AccountType: "regular", AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}
				cfg.tokenValidator = static
				if scenario == "chained static" {
					cfg.tokenValidator = chainedTokenValidator{static: static}
				}
			case "compat epoch":
				cfg.sessionEpochStrict = false
				want = http.StatusServiceUnavailable
			case "unbound issuer":
				cfg.tokenValidator = voicejwt.NewJWKSValidator(f.jwksURL, "", "voice-client")
			case "unbound audience":
				cfg.tokenValidator = voicejwt.NewJWKSValidator(f.jwksURL, "voice-auth", "")
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/chats", strings.NewReader(`{"type":"CHAT_TYPE_GROUP","name":"group"}`))
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-Voice-User-Id", f.payload["user_id"].(string))
			r.Header.Set("X-Voice-Profile-Id", f.payload["profile_id"].(string))
			r.Header.Set("X-Voice-Account-Type", "regular")
			switch scenario {
			case "query token":
				r.Header.Del("Authorization")
				r.URL.RawQuery = "access_token=" + token
			case "duplicate bearer":
				r.Header.Add("Authorization", "Bearer "+token)
			case "raw headers":
				r.Header.Del("Authorization")
			}
			w := httptest.NewRecorder()
			newGateway(cfg).ServeHTTP(w, r)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, calls)
		})
	}
}

func TestStandaloneChatCreationInvalidIDAndMissingTransport(t *testing.T) {
	f := newCreatorFixture(t)
	for _, scenario := range []string{"malformed", "nil UUID", "uppercase", "padded", "missing issuer", "missing protected"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			client := compositeChatClient{create: func(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
				calls++
				return nil, status.Error(codes.Unavailable, "unexpected dispatch")
			}}
			cfg := f.config(nil)
			cfg.transcoder.clients.chat, cfg.transcoder.clients.chatAudit = client, client
			id, want := "bad", http.StatusBadRequest
			switch scenario {
			case "nil UUID":
				id = uuid.Nil.String()
			case "uppercase":
				id = strings.ToUpper(uuid.NewString())
			case "padded":
				id = " " + uuid.NewString()
			case "missing issuer":
				id = uuid.NewString()
				cfg.transcoder.lifecycleIssuer = nil
				want = http.StatusServiceUnavailable
			case "missing protected":
				id = uuid.NewString()
				cfg.transcoder.clients.chatAudit = nil
				want = http.StatusServiceUnavailable
			}
			w := standaloneCreateRequest(t, cfg, f, f.payload, `{"type":"CHAT_TYPE_GROUP","request_id":"`+id+`"}`)
			require.Equal(t, want, w.Code, w.Body.String())
			if want == http.StatusBadRequest {
				require.Contains(t, w.Body.String(), "request_id must be a canonical UUID")
			}
			require.Zero(t, calls)
		})
	}
}

func TestStandaloneChatCreationNoRESTFallback(t *testing.T) {
	f := newCreatorFixture(t)
	for _, scenario := range []string{"missing transcoder", "missing clients", "protected only"} {
		t.Run(scenario, func(t *testing.T) {
			cfg := f.config(nil)
			protectedCalls, proxyCalls := 0, 0
			cfg.transcoder.clients.chatAudit = compositeChatClient{create: func(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
				protectedCalls++
				return &chatv1.CreateChatResponse{Chat: &chatv1.Chat{Id: uuid.NewString()}}, nil
			}}
			cfg.restUpstreams = map[string]http.Handler{"chats": http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				proxyCalls++
				w.WriteHeader(http.StatusAccepted)
			})}
			want := http.StatusServiceUnavailable
			switch scenario {
			case "missing transcoder":
				cfg.transcoder = nil
			case "missing clients":
				cfg.transcoder.clients.chatAudit = nil
			case "protected only":
				want = http.StatusOK
			}
			w := standaloneCreateRequest(t, cfg, f, f.payload, `{"type":"CHAT_TYPE_GROUP","name":"group"}`)
			require.Equal(t, want, w.Code, w.Body.String())
			require.Zero(t, proxyCalls)
			if scenario == "protected only" {
				require.Equal(t, 1, protectedCalls)
			} else {
				require.Zero(t, protectedCalls)
			}
		})
	}
}
