package botmembershipprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func TestBotMembershipEnvDefersOnlyItsBotJWKS(t *testing.T) {
	for _, name := range []string{"SPACE_BOT_MEMBERSHIP_TLS_CERT_FILE", "SPACE_BOT_MEMBERSHIP_TLS_KEY_FILE", "SPACE_BOT_MEMBERSHIP_CLIENT_CA_FILE", "SPACE_BOT_MEMBERSHIP_REPLAY_REDIS_ADDR"} {
		t.Setenv(name, "fixture")
	}
	t.Setenv("S2S_JWKS_URLS_JSON", `{"bot":"https://bot.test/jwks","role":"https://role.test/jwks"}`)
	cfg, enabled, err := LoadFromEnv()
	require.NoError(t, err)
	require.True(t, enabled)
	require.True(t, cfg.DeferInitialJWKS)
	require.Equal(t, map[string]string{"bot": "https://bot.test/jwks"}, cfg.JWKSURLs)
}

func TestBotMembershipDeferredJWKSBootstrapFailsClosedThenRecovers(t *testing.T) {
	cfg, publisher, _ := membershipRuntimeFixture(t)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var ready atomic.Bool
	var fetches atomic.Int64
	publisher.Config.Handler.(*http.ServeMux).HandleFunc("/bootstrap", func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		principal.JWKSHandlerKeys([]principal.JWKSKey{{KeyID: "current", PublicKey: &key.PublicKey}, {KeyID: "next", PublicKey: &next.PublicKey}}).ServeHTTP(w, r)
	})
	cfg.JWKSURLs["bot"] = publisher.URL + "/bootstrap"
	cfg.DeferInitialJWKS = true
	cfg.UnknownKIDCooldown = 10 * time.Millisecond
	runtime, err := New(t.Context(), cfg)
	require.NoError(t, err, "Bot publisher may start after healthy Space")
	defer runtime.Close()
	require.Zero(t, fetches.Load())
	intent := &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), InstallationId: uuid.NewString(), BotId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), Generation: 1}
	request := &spacev1.AddBotMemberRequest{SpaceId: intent.SpaceId, ProfileId: intent.ActorProfileId, InstallationIntent: intent}
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_AddBotMember_FullMethodName, RequestID: intent.OperationId, RequestHash: hash})
	require.NoError(t, err)
	calls := 0
	invoke := func() error {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", intent.OperationId))
		_, err := runtime.verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_AddBotMember_FullMethodName}, func(context.Context, any) (any, error) { calls++; return nil, nil })
		return err
	}
	require.Error(t, invoke())
	require.Zero(t, calls)
	require.Positive(t, fetches.Load())
	ready.Store(true)
	require.Eventually(t, func() bool { return invoke() == nil }, time.Second, 20*time.Millisecond)
	require.Equal(t, 1, calls)
	require.Error(t, invoke(), "recovered verifier still rejects JTI replay")
	require.Equal(t, 1, calls)
}
