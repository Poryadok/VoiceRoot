package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
	"voice/backend/role/internal/principalgrpc"
)

type signedSpaceDecisionVerifier struct{ config principal.VerifyConfig }

func (v signedSpaceDecisionVerifier) Verify(ctx context.Context, token, method, id, hash string) (principal.Principal, error) {
	cfg := v.config
	cfg.ExpectedRPC, cfg.ExpectedRequestID, cfg.ExpectedRequestHash = method, id, hash
	return principal.VerifyService(ctx, token, cfg)
}

// This legacy policy fixture enters through the actual signature verifier and
// protected interceptor. It does not manufacture a verified principal context.
func signedSpaceDecision(t *testing.T, svc *RoleGRPC, req *rolev1.CheckPermissionRequest) (*rolev1.CheckPermissionResponse, error) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	method := rolev1.RoleService_CheckPermission_FullMethodName
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "role", RPC: method, RequestID: id, RequestHash: hash})
	require.NoError(t, err)
	seen := make(map[string]bool)
	verifier := signedSpaceDecisionVerifier{principal.VerifyConfig{
		ExpectedIssuer: "space", ExpectedAudience: "role",
		KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
			if issuer != "space" || kid != "fixture" {
				return nil, fmt.Errorf("untrusted fixture key")
			}
			return &key.PublicKey, nil
		},
		ReplayGuard: func(_ context.Context, issuer, jwtID string, _ time.Time) error {
			if seen[issuer+jwtID] {
				return fmt.Errorf("replay")
			}
			seen[issuer+jwtID] = true
			return nil
		},
	}}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
	response, err := principalgrpc.StrictUnaryInterceptor(verifier)(ctx, req, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, request any) (any, error) {
		return svc.CheckPermission(ctx, request.(*rolev1.CheckPermissionRequest))
	})
	if err != nil {
		return nil, err
	}
	return response.(*rolev1.CheckPermissionResponse), nil
}
