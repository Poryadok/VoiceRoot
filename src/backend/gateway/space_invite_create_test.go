package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

type signedInviteClient struct {
	spacev1.SpaceServiceClient
	check func(context.Context, *spacev1.CreateInviteRequest)
	calls int
}

func (c *signedInviteClient) CreateInvite(ctx context.Context, r *spacev1.CreateInviteRequest, _ ...grpc.CallOption) (*spacev1.CreateInviteResponse, error) {
	c.calls++
	c.check(ctx, r)
	return &spacev1.CreateInviteResponse{}, nil
}

func TestCreateInvitePrivateNamespaceSignsVerifiedActor(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	space, account, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, mode := range []string{"valid", "raw only", "missing transport", "unknown body", "foreign space", "invalid limit"} {
		t.Run(mode, func(t *testing.T) {
			client := &signedInviteClient{check: func(ctx context.Context, r *spacev1.CreateInviteRequest) {
				require.Equal(t, space, r.SpaceId)
				require.EqualValues(t, 3, r.GetMaxUses())
				md, _ := metadata.FromOutgoingContext(ctx)
				require.Empty(t, md.Get("x-voice-profile-id"))
				hash, e := principal.RequestHash(r)
				require.NoError(t, e)
				p, e := principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{ExpectedIssuer: "gateway", ExpectedAudience: "space", ExpectedRPC: spacev1.SpaceService_CreateInvite_FullMethodName, ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash, KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }, SessionEpochChecker: func(context.Context, string, int64) error { return nil }})
				require.NoError(t, e)
				require.Equal(t, profile, p.ProfileID)
				require.Equal(t, account, p.AccountID)
				require.Equal(t, "regular", p.AccountType)
			}}
			tr := &transcoder{clients: grpcClients{spaceLifecycle: client}, lifecycleIssuer: issuer}
			if mode == "missing transport" {
				tr.clients.spaceLifecycle = nil
			}
			body := `{"maxUses":3}`
			if mode == "unknown body" {
				body = `{"actor":"forged"}`
			}
			if mode == "foreign space" {
				body = `{"spaceId":"` + uuid.NewString() + `"}`
			}
			if mode == "invalid limit" {
				body = `{"maxUses":0}`
			}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/"+space+"/invites", strings.NewReader(body))
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
