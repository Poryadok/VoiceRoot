package roleoverrideprincipal

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"voice/backend/pkg/principal"
)

func TestMemberAdmissionAuthorityUsesOnlyProtectedRoleSpaceRoute(t *testing.T) {
	const method = "/voice.space.v1.SpaceService/GetMemberAdmissionAuthority"
	req := &emptypb.Empty{}
	hash, err := principal.RequestHash(req)
	require.NoError(t, err)
	id := uuid.NewString()
	p := principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: "space", RPC: method, RequestID: id, RequestHash: hash}
	v := verifierFunc(func(context.Context, string, string, string, string) (principal.Principal, error) { return p, nil })
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", id))
	info := &grpc.UnaryServerInfo{FullMethod: method}
	reached := false
	handler := func(context.Context, any) (any, error) { reached = true; return nil, nil }
	_, err = StrictUnaryInterceptor("space", v)(ctx, req, info, handler)
	require.NoError(t, err)
	require.True(t, reached)
	for _, target := range []string{"space", "chat"} {
		reached = false
		_, err = OrdinaryUnaryInterceptor(target)(ctx, req, info, handler)
		if target == "space" {
			require.Equal(t, codes.Unavailable, status.Code(err))
			require.False(t, reached)
		}
	}
	reached = false
	_, err = StrictUnaryInterceptor("chat", v)(ctx, req, info, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, reached)
	p.Issuer, p.Subject = "bot", "service:bot"
	_, err = StrictUnaryInterceptor("space", v)(ctx, req, info, handler)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, reached)
	p.Issuer, p.Subject, p.RequestHash = "role", "service:role", "changed"
	_, err = StrictUnaryInterceptor("space", v)(ctx, req, info, handler)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, reached)
}
