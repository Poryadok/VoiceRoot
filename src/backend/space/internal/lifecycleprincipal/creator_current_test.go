package lifecycleprincipal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	spacev1 "voice.app/voice/space/v1"
)

func TestCurrentOrdinaryCreateSpaceIsDenied(t *testing.T) {
	called := false
	_, err := OrdinaryUnary()(context.Background(), &spacev1.CreateSpaceRequest{Name: "Creator"}, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.False(t, called)
}

func TestCurrentCreatorRequiresDedicatedRuntimeDependencies(t *testing.T) {
	called := false
	_, err := (Verifier{}).Unary()(context.Background(), &spacev1.CreateSpaceRequest{Name: "Creator"}, &grpc.UnaryServerInfo{FullMethod: spacev1.SpaceService_CreateSpace_FullMethodName}, func(context.Context, any) (any, error) {
		called = true
		return nil, nil
	})
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.False(t, called)
}
