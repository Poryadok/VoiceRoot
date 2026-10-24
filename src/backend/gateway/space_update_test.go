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

type signedSpaceUpdateClient struct {
	spacev1.SpaceServiceClient
	check func(context.Context, *spacev1.UpdateSpaceRequest)
	calls int
}

type signedSpaceMMUpdateClient struct {
	spacev1.SpaceServiceClient
	check func(context.Context, *spacev1.UpdateSpaceMmConfigRequest)
}

func (c *signedSpaceMMUpdateClient) UpdateSpaceMmConfig(ctx context.Context, r *spacev1.UpdateSpaceMmConfigRequest, _ ...grpc.CallOption) (*spacev1.UpdateSpaceMmConfigResponse, error) {
	c.check(ctx, r)
	return &spacev1.UpdateSpaceMmConfigResponse{}, nil
}

func TestUpdateSpaceMMPrivateNamespaceSignsVerifiedActor(t *testing.T) {
	f := newCreatorFixture(t)
	space := uuid.NewString()
	for _, mode := range []string{"valid", "foreign space", "unknown body", "query", "missing transport"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			client := &signedSpaceMMUpdateClient{check: func(ctx context.Context, req *spacev1.UpdateSpaceMmConfigRequest) {
				calls++
				require.Equal(t, space, req.SpaceId)
				require.Equal(t, `{"enabled":true}`, req.MmConfigJson)
				md, _ := metadata.FromOutgoingContext(ctx)
				hash, err := principal.RequestHash(req)
				require.NoError(t, err)
				p, err := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
					ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_UpdateSpaceMmConfig_FullMethodName,
					ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
					KeyResolver:         func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
					ReplayGuard:         func(context.Context, string, string, time.Time) error { return nil },
					SessionEpochChecker: func(context.Context, string, int64) error { return nil },
				})
				require.NoError(t, err)
				require.Equal(t, f.payload["profile_id"], p.ProfileID)
			}}
			cfg := f.config(client)
			cfg.transcoder.clients.space = nil
			if mode == "missing transport" {
				cfg.transcoder.clients.spaceLifecycle = nil
			}
			body := `{"mmConfigJson":"{\"enabled\":true}"}`
			path := "/api/v1/spaces/" + space + "/matchmaking/config"
			if mode == "foreign space" {
				body = `{"spaceId":"` + uuid.NewString() + `"}`
			}
			if mode == "unknown body" {
				body = `{"actor":"forged"}`
			}
			if mode == "query" {
				path += "?actor=forged"
			}
			w := performRequest(newGateway(cfg), http.MethodPatch, path, body, map[string]string{"Authorization": "Bearer " + signConfigJWT(t, "creator-key", f.key, f.payload)})
			if mode == "valid" {
				require.Equal(t, 200, w.Code, w.Body.String())
				require.Equal(t, 1, calls)
			} else {
				require.GreaterOrEqual(t, w.Code, 400)
				require.Zero(t, calls)
			}
		})
	}
}

func (c *signedSpaceUpdateClient) UpdateSpace(ctx context.Context, r *spacev1.UpdateSpaceRequest, _ ...grpc.CallOption) (*spacev1.UpdateSpaceResponse, error) {
	c.calls++
	c.check(ctx, r)
	return &spacev1.UpdateSpaceResponse{}, nil
}

func TestUpdateSpacePrivateNamespaceSignsVerifiedActor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	space, account, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, mode := range []string{"valid", "raw only", "missing transport", "unknown body", "foreign space"} {
		t.Run(mode, func(t *testing.T) {
			client := &signedSpaceUpdateClient{check: func(ctx context.Context, r *spacev1.UpdateSpaceRequest) {
				require.Equal(t, space, r.SpaceId)
				require.Equal(t, "Updated", r.GetName())
				require.Equal(t, "https://example.test/icon", r.GetIconUrl())
				require.Equal(t, "description", r.GetDescription())
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Empty(t, md.Get("x-voice-profile-id"))
				hash, e := principal.RequestHash(r)
				require.NoError(t, e)
				p, e := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
					ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_UpdateSpace_FullMethodName,
					ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
					KeyResolver:         func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil },
					ReplayGuard:         func(context.Context, string, string, time.Time) error { return nil },
					SessionEpochChecker: func(context.Context, string, int64) error { return nil },
				})
				require.NoError(t, e)
				require.Equal(t, profile, p.ProfileID)
				require.Equal(t, account, p.AccountID)
			}}
			tr := &transcoder{clients: grpcClients{spaceLifecycle: client}, lifecycleIssuer: issuer}
			if mode == "missing transport" {
				tr.clients.spaceLifecycle = nil
			}
			body := `{"name":"Updated","iconUrl":"https://example.test/icon","description":"description"}`
			if mode == "unknown body" {
				body = `{"actor":"forged"}`
			}
			if mode == "foreign space" {
				body = `{"spaceId":"` + uuid.NewString() + `"}`
			}
			r := httptest.NewRequest(http.MethodPatch, "/api/v1/spaces/"+space, strings.NewReader(body))
			r.Header.Set("x-voice-profile-id", uuid.NewString())
			if mode != "raw only" {
				r = r.WithContext(context.WithValue(r.Context(), verifiedUserClaimsKey{}, tokenClaims{UserID: account, ProfileID: profile, SessionEpoch: 7, ExpiresAt: time.Now().Add(time.Minute), AuthenticatedAccountType: "regular"}))
			}
			w := httptest.NewRecorder()
			require.True(t, tr.serveNamespace(w, r, "spaces"))
			if mode == "valid" {
				require.Equal(t, 200, w.Code)
				require.Equal(t, 1, client.calls)
			} else {
				require.GreaterOrEqual(t, w.Code, 400)
				require.Zero(t, client.calls)
			}
		})
	}
}
