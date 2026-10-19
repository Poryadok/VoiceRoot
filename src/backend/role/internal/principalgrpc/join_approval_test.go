package principalgrpc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"testing"
	rolev1 "voice.app/voice/role/v1"
	"voice/backend/pkg/principal"
)

func TestJoinApprovalProtectedExactSpaceDecision(t *testing.T) {
	req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "SPACE_APPROVE_JOIN_REQUESTS"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer verified-by-fixture", "x-request-id", uuid.NewString()))
	intercept := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, _ string, method, id, hash string) (principal.Principal, error) {
		return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: id, RequestHash: hash}, nil
	}))
	called := false
	_, err := intercept(ctx, req, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.NoError(t, err)
	require.True(t, called)
}

func TestSettingsProtectedExactSpaceDecision(t *testing.T) {
	req := &rolev1.CheckPermissionRequest{SpaceId: uuid.NewString(), ProfileId: uuid.NewString(), PermissionName: "SPACE_MANAGE_SETTINGS"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer verified-by-fixture", "x-request-id", uuid.NewString()))
	intercept := StrictUnaryInterceptor(verifierFunc(func(_ context.Context, _ string, method, id, hash string) (principal.Principal, error) {
		return principal.Principal{Kind: "service", Issuer: "space", Subject: "service:space", Audience: "role", RPC: method, RequestID: id, RequestHash: hash}, nil
	}))
	called := false
	_, err := intercept(ctx, req, &grpc.UnaryServerInfo{FullMethod: rolev1.RoleService_CheckPermission_FullMethodName}, func(context.Context, any) (any, error) { called = true; return nil, nil })
	require.NoError(t, err)
	require.True(t, called)
}
