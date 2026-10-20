package store

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/principal"
	"voice/backend/space/internal/botmembershipprincipal"
)

func signedNativeP3ChildCaller(t *testing.T, st *SpaceStore) func(*spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.BotPurgeInstallationReceipt, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "bot", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	verifier := botmembershipprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }}
	return func(request *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.BotPurgeInstallationReceipt, error) {
		hash, err := principal.RequestHash(request)
		if err != nil {
			return nil, err
		}
		token, err := issuer.IssueService(principal.ServiceInput{Audience: "space", RPC: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName, RequestID: request.OperationId, RequestHash: hash})
		if err != nil {
			return nil, err
		}
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", request.OperationId))
		result, err := verifier.Unary()(ctx, request, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_ReconcileBotPurgeInstallation_FullMethodName}, func(ctx context.Context, _ any) (any, error) {
			proof, err := botmembershipprincipal.RequirePurgeInstallation(ctx, request)
			if err != nil {
				return nil, err
			}
			implementation, ok := any(st).(interface {
				ReconcileBotPurgeInstallation(context.Context, *botmembershipprincipal.VerifiedPurgeInstallation, *spacev1.ReconcileBotPurgeInstallationRequest) (*spacev1.BotPurgeInstallationReceipt, error)
			})
			if !ok {
				return nil, status.Error(codes.Unimplemented, "native P3 child Store is not implemented")
			}
			return implementation.ReconcileBotPurgeInstallation(ctx, proof, request)
		})
		if err != nil {
			return nil, err
		}
		return result.(*spacev1.BotPurgeInstallationReceipt), nil
	}
}
