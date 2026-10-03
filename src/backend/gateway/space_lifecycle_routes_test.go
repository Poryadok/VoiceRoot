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
	"google.golang.org/grpc/metadata"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type publicLifecycleClient struct {
	spacev1.SpaceServiceClient
	verify func(context.Context, any, string)
	calls  int
}

func (c *publicLifecycleClient) DeleteSpace(ctx context.Context, req *spacev1.DeleteSpaceRequest, _ ...grpc.CallOption) (*spacev1.DeleteSpaceResponse, error) {
	c.calls++
	c.verify(ctx, req, spacev1.SpaceService_DeleteSpace_FullMethodName)
	return &spacev1.DeleteSpaceResponse{}, nil
}
func (c *publicLifecycleClient) RestoreSpace(ctx context.Context, req *spacev1.RestoreSpaceRequest, _ ...grpc.CallOption) (*spacev1.RestoreSpaceResponse, error) {
	c.calls++
	c.verify(ctx, req, spacev1.SpaceService_RestoreSpace_FullMethodName)
	return &spacev1.RestoreSpaceResponse{Space: &spacev1.Space{Id: req.SpaceId, Name: "restored"}}, nil
}

func (c *publicLifecycleClient) GetSpace(ctx context.Context, req *spacev1.GetSpaceRequest, _ ...grpc.CallOption) (*spacev1.GetSpaceResponse, error) {
	c.calls++
	c.verify(ctx, req, spacev1.SpaceService_GetSpace_FullMethodName)
	return &spacev1.GetSpaceResponse{Space: &spacev1.Space{Id: req.SpaceId, Name: "minimal owner projection"}}, nil
}

func TestPublicLifecycleRoutesSignExactVerifiedActorAndRejectInvalidIntents(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	space, op, account, profile := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	deleteBody := `{"confirmation_name":"Exact Name","proof":"opaque-proof","operation_id":"` + op + `"}`
	restoreBody := `{"operation_id":"` + op + `"}`
	for _, tc := range []struct {
		name, method, path, body, accountType string
		code                                  int
	}{
		{"delete", http.MethodDelete, "", deleteBody, "regular", 204},
		{"restore", http.MethodPost, "/restore", restoreBody, "regular", 200},
		{"matching path body", http.MethodDelete, "", strings.TrimSuffix(deleteBody, "}") + `,"space_id":"` + space + `"}`, "regular", 204},
		{"foreign path body", http.MethodPost, "/restore", strings.TrimSuffix(restoreBody, "}") + `,"spaceId":"` + uuid.NewString() + `"}`, "regular", 400},
		{"duplicate alias", http.MethodPost, "/restore", strings.TrimSuffix(restoreBody, "}") + `,"operationId":"` + op + `"}`, "regular", 400},
		{"unknown", http.MethodDelete, "", strings.TrimSuffix(deleteBody, "}") + `,"actor_id":"` + profile + `"}`, "regular", 400},
		{"missing proof", http.MethodDelete, "", `{"operation_id":"` + op + `"}`, "regular", 400},
		{"invalid operation", http.MethodPost, "/restore", `{"operation_id":"bad"}`, "regular", 400},
		{"sdk", http.MethodPost, "/restore", restoreBody, "sdk-account", 403},
		{"guest write", http.MethodDelete, "", deleteBody, "guest", 403},
		{"regular read", http.MethodGet, "", "", "regular", 200},
		{"sdk read", http.MethodGet, "", "", "sdk-account", 200},
		{"guest read", http.MethodGet, "", "", "guest", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client := &publicLifecycleClient{}
			client.verify = func(ctx context.Context, req any, method string) {
				md, ok := metadata.FromOutgoingContext(ctx)
				require.True(t, ok)
				require.Empty(t, md.Get("x-voice-user-id"))
				require.Empty(t, md.Get("x-voice-profile-id"))
				require.Empty(t, md.Get("x-voice-session-epoch"))
				require.Len(t, md.Get("x-request-id"), 1)
				requestID := md.Get("x-request-id")[0]
				if method == spacev1.SpaceService_GetSpace_FullMethodName {
					require.True(t, canonicalLifecycleUUID(requestID))
				} else {
					require.Equal(t, op, requestID)
				}
				var hash string
				switch q := req.(type) {
				case *spacev1.DeleteSpaceRequest:
					require.Equal(t, space, q.SpaceId)
					hash, err = principal.RequestHash(q)
				case *spacev1.RestoreSpaceRequest:
					require.Equal(t, space, q.SpaceId)
					hash, err = principal.RequestHash(q)
				case *spacev1.GetSpaceRequest:
					require.Equal(t, space, q.SpaceId)
					hash, err = principal.RequestHash(q)
				}
				require.NoError(t, err)
				p, err := principal.VerifyDelegatedUser(context.Background(), strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: method, ExpectedRequestID: requestID, ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(_ context.Context, a string, e int64) error {
					require.Equal(t, account, a)
					require.Equal(t, int64(7), e)
					return nil
				}})
				require.NoError(t, err)
				require.Equal(t, profile, p.ProfileID)
			}
			h := newGateway(gatewayConfig{tokenClaims: map[string]tokenClaims{"user": {UserID: account, ProfileID: profile, SessionEpoch: 7, AccountType: tc.accountType, ExpiresAt: time.Now().Add(time.Minute)}}, transcoder: &transcoder{clients: grpcClients{spaceLifecycle: client}, lifecycleIssuer: issuer}})
			r := httptest.NewRequest(tc.method, "/api/v1/spaces/"+space+tc.path, strings.NewReader(tc.body))
			r.Header.Set("Authorization", "Bearer user")
			r.Header.Set("X-Voice-User-Id", uuid.NewString())
			r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
			r.Header.Set("X-Voice-Session-Epoch", "99")
			r.Header.Set("X-Voice-Account-Type", "regular")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			require.Equal(t, tc.code, w.Code, w.Body.String())
			if tc.code == 200 || tc.code == 204 {
				require.Equal(t, 1, client.calls)
			} else {
				require.Zero(t, client.calls)
			}
			if tc.code == 204 {
				require.Empty(t, w.Body.String())
			}
		})
	}
}
