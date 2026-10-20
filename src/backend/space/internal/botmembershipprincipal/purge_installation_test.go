package botmembershipprincipal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	commonv1 "voice.app/voice/common/v1"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
)

func nativePurgeRequest() *spacev1.ReconcileBotPurgeInstallationRequest {
	space, bot, profile, installation := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	return &spacev1.ReconcileBotPurgeInstallationRequest{ProtocolVersion: 1, OperationId: uuid.NewString(), SpaceId: space, DeletionOperationId: uuid.NewString(), Generation: 2,
		Manifest: &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ManifestSha256: make([]byte, 32), ItemCount: 1}, BotId: bot, InstallationId: installation,
		NativeCancellation: &spacev1.RemoveBotMemberRequest{SpaceId: space, ProfileId: profile, InstallOperationId: uuid.NewString(), InstallationIntent: &spacev1.BotInstallationIntent{OperationId: uuid.NewString(), BotId: bot, InstallationId: installation, SpaceId: space, ActorProfileId: profile, Generation: 1}}}
}

func TestBotPurgeInstallationRequiresOwnProtectedRoute(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now().UTC()
	verifier := Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Clock: func() time.Time { return now }}
	request := nativePurgeRequest()
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key, Clock: func() time.Time { return now }})
	require.NoError(t, err)
	hash, err := principal.RequestHash(request)
	require.NoError(t, err)
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
	require.NoError(t, err)
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
	t.Run("exact pure Bot protected method", func(t *testing.T) {
		called := false
		_, err := verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.NoError(t, err)
		require.True(t, called)
	})
	t.Run("ordinary method denied", func(t *testing.T) {
		called := false
		_, err := OrdinaryUnary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
		require.Error(t, err)
		require.False(t, called)
	})
}
