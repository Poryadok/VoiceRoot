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
	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/principal"
)

func TestAuditDelegationUsesRawParsedAccountType(t *testing.T) {
	f := newCreatorFixture(t)
	for _, kind := range []string{"regular", "guest", "sdk-account", "missing", "empty", " regular ", "service"} {
		t.Run(kind, func(t *testing.T) {
			payload := cloneConfigJWTPayload(f.payload)
			payload["account_type"] = kind
			switch kind {
			case "missing":
				delete(payload, "account_type")
			case "empty":
				payload["account_type"] = ""
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+uuid.NewString()+"/chats", nil)
			r.Header.Set("Authorization", "Bearer "+signConfigJWT(t, "creator-key", f.key, payload))
			claims, code := f.validator.Validate(r)
			require.Empty(t, code)
			if kind == "missing" || kind == "empty" || kind == " regular " {
				require.Equal(t, "regular", claims.AccountType, "exercise the real compatibility default/trim")
			}
			applyClaims(r, claims)
			r.Header.Set("X-Voice-Account-Type", "regular") // untrusted override cannot repair the signed claim
			tr := &transcoder{lifecycleIssuer: f.issuer, clients: grpcClients{chatAudit: chatv1.NewChatServiceClient(nil), spaceLifecycle: spacev1.NewSpaceServiceClient(nil)}}
			auditReq := &chatv1.CreateChatRequest{RequestId: uuid.NewString(), Type: chatv1.ChatType_CHAT_TYPE_GROUP}
			outcomeReq := &spacev1.GetSpaceChatCreationOutcomeRequest{SpaceId: uuid.NewString(), OperationId: uuid.NewString()}
			for _, composite := range []bool{false, true} {
				var ctx context.Context
				var err error
				audience, method, id := "chat", chatv1.ChatService_CreateChat_FullMethodName, auditReq.RequestId
				hash, hashErr := principal.RequestHash(auditReq)
				require.NoError(t, hashErr)
				if composite {
					ctx, err = tr.compositeOutcomeContext(r, outcomeReq)
					audience, method, id = "space", spacev1.SpaceService_GetSpaceChatCreationOutcome_FullMethodName, outcomeReq.OperationId
					hash, hashErr = principal.RequestHash(outcomeReq)
					require.NoError(t, hashErr)
				} else {
					ctx, err = tr.auditInitiatingContext(r, auditReq, method, id)
				}
				allowed := kind == "regular" || kind == "guest" || !composite && kind == "sdk-account"
				if !allowed {
					require.Equal(t, codes.Unauthenticated, status.Code(err), "composite=%t", composite)
					require.Nil(t, ctx)
					continue
				}
				require.NoError(t, err)
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				verified, err := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
					ExpectedIssuer: "gateway", ExpectedAudience: audience, ExpectedRPC: method, ExpectedRequestID: id, ExpectedRequestHash: hash,
					KeyResolver:         func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
					SessionEpochChecker: func(context.Context, string, int64) error { return nil },
				})
				require.NoError(t, err)
				require.Equal(t, kind, verified.AccountType)
			}
		})
	}
}

func TestSpaceChatDelegationUsesStrictJWTBoundary(t *testing.T) {
	f := newCreatorFixture(t)
	for _, scenario := range []string{"regular", "guest", "missing type", "trimmed type", "missing jti", "wrong issuer", "wrong audience", "unconfigured issuer", "unconfigured audience", "query token", "duplicate bearer", "static", "chained static", "compat epoch"} {
		t.Run(scenario, func(t *testing.T) {
			payload := cloneConfigJWTPayload(f.payload)
			switch scenario {
			case "guest":
				payload["account_type"] = "guest"
			case "missing type":
				delete(payload, "account_type")
			case "trimmed type":
				payload["account_type"] = " regular "
			case "missing jti":
				delete(payload, "jti")
			case "wrong issuer":
				payload["iss"] = "untrusted"
			case "wrong audience":
				payload["aud"] = "untrusted"
			}
			token := signConfigJWT(t, "creator-key", f.key, payload)
			calls := 0
			cfg := f.config(spacev1.NewSpaceServiceClient(nil))
			cfg.transcoder.clients.chatAudit = compositeChatClient{create: func(context.Context, *chatv1.CreateChatRequest) (*chatv1.CreateChatResponse, error) {
				calls++
				return nil, status.Error(codes.Unavailable, "stop after authenticated dispatch")
			}}
			cfg.transcoder.clients.chat = nil
			switch scenario {
			case "unconfigured issuer":
				cfg.tokenValidator = voicejwt.NewJWKSValidator(f.jwksURL, "", "voice-client")
			case "unconfigured audience":
				cfg.tokenValidator = voicejwt.NewJWKSValidator(f.jwksURL, "voice-auth", "")
			case "static", "chained static":
				static := staticTokenValidator{token: {UserID: payload["user_id"].(string), ProfileID: payload["profile_id"].(string), SessionEpoch: 7, AccountType: "regular", AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}}
				cfg.tokenValidator = static
				if scenario == "chained static" {
					cfg.tokenValidator = chainedTokenValidator{static: static}
				}
			case "compat epoch":
				cfg.sessionEpochStrict = false
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+uuid.NewString()+"/chats", strings.NewReader(`{"type":"CHAT_TYPE_GROUP","name":"group"}`))
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-Voice-Account-Type", "regular")
			switch scenario {
			case "query token":
				r.Header.Del("Authorization")
				r.URL.RawQuery = "access_token=" + token
			case "duplicate bearer":
				r.Header.Add("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			newGateway(cfg).ServeHTTP(w, r)
			if scenario == "regular" || scenario == "guest" {
				require.Equal(t, http.StatusServiceUnavailable, w.Code, w.Body.String())
				require.Equal(t, 1, calls)
			} else {
				want := http.StatusUnauthorized
				if scenario == "compat epoch" {
					want = http.StatusServiceUnavailable
				}
				require.Equal(t, want, w.Code, w.Body.String())
				require.Zero(t, calls, "invalid capability input must not dispatch")
			}
		})
	}
}
