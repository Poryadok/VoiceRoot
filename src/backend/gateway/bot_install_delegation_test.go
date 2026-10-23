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
	"google.golang.org/protobuf/proto"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

type delegatedBotInstallationClient struct {
	botv1.BotServiceClient
	call func(context.Context, proto.Message, string)
}

func (c *delegatedBotInstallationClient) InstallBotInSpace(ctx context.Context, req *botv1.InstallBotInSpaceRequest, _ ...grpc.CallOption) (*botv1.InstallBotInSpaceResponse, error) {
	c.call(ctx, req, botv1.BotService_InstallBotInSpace_FullMethodName)
	return &botv1.InstallBotInSpaceResponse{}, nil
}

func (c *delegatedBotInstallationClient) UninstallBotFromSpace(ctx context.Context, req *botv1.UninstallBotFromSpaceRequest, _ ...grpc.CallOption) (*botv1.UninstallBotFromSpaceResponse, error) {
	c.call(ctx, req, botv1.BotService_UninstallBotFromSpace_FullMethodName)
	return &botv1.UninstallBotFromSpaceResponse{}, nil
}

func TestBotInstallationUsesProtectedDelegation(t *testing.T) {
	f := newCreatorFixture(t)
	for _, install := range []bool{true, false} {
		account, profile, bot, space := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
		method, rest, body, want := http.MethodDelete, bot+"/spaces/"+space, "", http.StatusNoContent
		if install {
			method, rest, body, want = http.MethodPost, rest+"/install", `{}`, http.StatusOK
		}
		called := false
		client := &delegatedBotInstallationClient{call: func(ctx context.Context, req proto.Message, rpc string) {
			called = true
			md, ok := metadata.FromOutgoingContext(ctx)
			require.True(t, ok)
			require.Len(t, md.Get("authorization"), 1)
			hash, err := principal.RequestHash(req)
			require.NoError(t, err)
			_, err = principal.VerifyDelegatedUser(ctx, strings.TrimPrefix(md.Get("authorization")[0], "Bearer "), principal.VerifyConfig{
				ExpectedIssuer: "gateway", ExpectedAudience: "bot", ExpectedRPC: rpc,
				ExpectedRequestID: md.Get("x-request-id")[0], ExpectedRequestHash: hash,
				KeyResolver: func(context.Context, string, string) (*rsa.PublicKey, error) { return &f.key.PublicKey, nil },
				SessionEpochChecker: func(_ context.Context, id string, epoch int64) error {
					require.Equal(t, account, id)
					require.EqualValues(t, 7, epoch)
					return nil
				},
			})
			require.NoError(t, err)
			switch r := req.(type) {
			case *botv1.InstallBotInSpaceRequest:
				require.Equal(t, bot, r.BotId)
				require.Equal(t, space, r.SpaceId)
			case *botv1.UninstallBotFromSpaceRequest:
				require.Equal(t, bot, r.BotId)
				require.Equal(t, space, r.SpaceId)
			}
		}}
		tr := &transcoder{lifecycleIssuer: f.issuer, clients: grpcClients{botAuditAdmin: client}}
		request := func() *http.Request {
			r := httptest.NewRequest(method, "/api/v1/bots/"+rest, strings.NewReader(body))
			applyClaims(r, tokenClaims{UserID: account, ProfileID: profile, SessionEpoch: 7, AuthenticatedAccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)})
			return r
		}
		w := httptest.NewRecorder()
		require.True(t, tr.serveNamespace(w, request(), "bots"))
		require.Equal(t, want, w.Code, w.Body.String())
		require.True(t, called)
		tr.clients.botAuditAdmin = nil
		w = httptest.NewRecorder()
		require.True(t, tr.serveBots(w, request(), rest))
		require.Equal(t, http.StatusServiceUnavailable, w.Code)
	}
}
