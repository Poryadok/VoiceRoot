package roleoverrideprincipal

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"testing"
	"voice/backend/pkg/principal"
)

type verifierFunc func(context.Context, string, string, string, string) (principal.Principal, error)

func (f verifierFunc) Verify(ctx context.Context, a, b, c, d string) (principal.Principal, error) {
	return f(ctx, a, b, c, d)
}

func TestStrictBoundaryRejectsBeforeDomain(t *testing.T) {
	for _, target := range []string{"chat", "space"} {
		req := &emptypb.Empty{}
		hash, err := principal.RequestHash(req)
		require.NoError(t, err)
		id := uuid.NewString()
		p := principal.Principal{Kind: "service", Issuer: "role", Subject: "service:role", Audience: target, RPC: Method(target), RequestID: id, RequestHash: hash}
		calls := 0
		v := verifierFunc(func(_ context.Context, _, method, requestID, bodyHash string) (principal.Principal, error) {
			calls++
			require.Equal(t, p.RPC, method)
			require.Equal(t, id, requestID)
			require.Equal(t, hash, bodyHash)
			return p, nil
		})
		interceptor := StrictUnaryInterceptor(target, v)
		info := &grpc.UnaryServerInfo{FullMethod: Method(target)}
		reached := false
		handler := func(ctx context.Context, _ any) (any, error) { reached = true; return nil, Require(ctx, target, req) }
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer token", "x-request-id", id))
		_, err = interceptor(ctx, req, info, handler)
		require.NoError(t, err)
		require.True(t, reached)
		for _, pairs := range [][]string{{"x-request-id", id}, {"authorization", "Bearer token", "authorization", "Bearer other", "x-request-id", id}, {"authorization", "Bearer token", "x-request-id", id, "x-request-id", id}, {"authorization", "Bearer token", "x-request-id", id, "x-voice-profile-id", uuid.NewString()}} {
			before := calls
			reached = false
			_, err = interceptor(metadata.NewIncomingContext(context.Background(), metadata.Pairs(pairs...)), req, info, handler)
			require.Equal(t, codes.Unauthenticated, status.Code(err))
			require.False(t, reached)
			require.Equal(t, before, calls)
		}
		reached = false
		_, err = interceptor(ctx, req, &grpc.UnaryServerInfo{FullMethod: "/other/Method"}, handler)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.False(t, reached)
		p.Issuer = "space"
		p.Subject = "service:space"
		reached = false
		_, err = interceptor(ctx, req, info, handler)
		require.Equal(t, codes.PermissionDenied, status.Code(err))
		require.False(t, reached)
		p.Issuer = "role"
		p.Subject = "service:role"
		p.RequestHash = "changed"
		reached = false
		_, err = interceptor(ctx, req, info, handler)
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.False(t, reached)
		reached = false
		_, err = OrdinaryUnaryInterceptor(target)(ctx, req, info, handler)
		require.Equal(t, codes.Unavailable, status.Code(err))
		require.False(t, reached)
	}
}
