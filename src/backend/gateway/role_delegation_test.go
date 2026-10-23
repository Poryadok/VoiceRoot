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
	"google.golang.org/grpc/metadata"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

type delegatedRoleListClient struct {
	rolev1.RoleServiceClient
	call func(context.Context, *rolev1.ListRolesRequest)
}

func (c *delegatedRoleListClient) ListRoles(ctx context.Context, req *rolev1.ListRolesRequest, _ ...grpc.CallOption) (*rolev1.ListRolesResponse, error) {
	c.call(ctx, req)
	return &rolev1.ListRolesResponse{}, nil
}

func TestRoleListUsesProtectedDelegation(t *testing.T) {
	f := newCreatorFixture(t)
	account, profile, space := uuid.NewString(), uuid.NewString(), uuid.NewString()
	called := false
	client := &delegatedRoleListClient{call: func(ctx context.Context, req *rolev1.ListRolesRequest) {
		called = true
		require.Equal(t, space, req.SpaceId)
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		require.Empty(t, md.Get("x-profile-id"))
		require.Len(t, md.Get("authorization"), 1)
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		_, err = principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
			ExpectedIssuer: "gateway", ExpectedAudience: "role", ExpectedRPC: rolev1.RoleService_ListRoles_FullMethodName,
			ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
			KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
			SessionEpochChecker: func(_ context.Context, id string, epoch int64) error {
				require.Equal(t, account, id)
				require.EqualValues(t, 7, epoch)
				return nil
			},
		})
		require.NoError(t, err)
	}}
	tr := &transcoder{lifecycleIssuer: f.issuer, clients: grpcClients{roleAudit: client}}
	r := httptest.NewRequest(http.MethodGet, "/api/v1/roles?space_id="+space, nil)
	applyClaims(r, tokenClaims{UserID: account, ProfileID: profile, SessionEpoch: 7, AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)})
	r.Header.Set("X-Profile-ID", uuid.NewString())
	w := httptest.NewRecorder()
	require.True(t, tr.serveNamespace(w, r, "roles"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, called)
	tr.clients.roleAudit = nil
	tr.clients.role = client
	w = httptest.NewRecorder()
	called = false
	require.True(t, tr.serveRoles(w, r, ""))
	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	require.False(t, called, "missing private transport must not use the ordinary client")
}

func TestProtectedRoleAndBotRoutesNeverProxyWithoutTransport(t *testing.T) {
	for _, route := range []struct{ method, path, namespace string }{
		{http.MethodGet, "/api/v1/roles?space_id=x", "roles"},
		{http.MethodPost, "/api/v1/roles/assign", "roles"},
		{http.MethodPost, "/api/v1/bots/b/spaces/s/install", "bots"},
		{http.MethodDelete, "/api/v1/bots/b/spaces/s", "bots"},
	} {
		for _, tr := range []*transcoder{nil, {}} {
			proxied := false
			h := newGatewayForContract(t, gatewayTestOptions{
				tokenClaims:   map[string]tokenClaims{"valid": {UserID: uuid.NewString(), ProfileID: uuid.NewString(), SessionEpoch: 7, AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}},
				transcoder:    tr,
				restUpstreams: map[string]http.Handler{route.namespace: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { proxied = true; w.WriteHeader(http.StatusAccepted) })},
			})
			response := performRequest(h, route.method, route.path, `{}`, map[string]string{"Authorization": "Bearer valid"})
			require.Equal(t, http.StatusServiceUnavailable, response.Code, route.path)
			require.False(t, proxied, route.path)
		}
	}
}
