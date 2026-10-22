package chatcreation

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	authv1 "voice.app/voice/auth/v1"
	"voice/backend/pkg/principal"
)

type floorClientFunc func(context.Context, *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error)

func (f floorClientFunc) GetChatSessionEpochFloor(ctx context.Context, r *authv1.GetChatSessionEpochFloorRequest, _ ...grpc.CallOption) (*authv1.GetChatSessionEpochFloorResponse, error) {
	return f(ctx, r)
}

func TestAuthFloorCheckerReadsEveryCallAndFailsClosed(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "chat", KeyID: "current", PrivateKey: key})
	require.NoError(t, err)
	account, id := uuid.NewString(), uuid.NewString()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer delegated-credential", "x-request-id", id))
	var floor int64 = 9
	var remoteErr error
	calls := 0
	checker := &AuthFloorChecker{Issuer: issuer, Client: floorClientFunc(func(call context.Context, r *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error) {
		calls++
		require.Equal(t, account, r.AccountId)
		md, _ := metadata.FromOutgoingContext(call)
		require.Equal(t, []string{id}, md.Get("x-request-id"))
		require.NotEqual(t, []string{"Bearer delegated-credential"}, md.Get("authorization"))
		return &authv1.GetChatSessionEpochFloorResponse{SessionEpochFloor: floor}, remoteErr
	})}
	require.NoError(t, checker.CheckSessionEpoch(ctx, account, 9), "equality is accepted")
	floor = 10
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 9), ErrInitiator, "fresh floor invalidates prior success")
	require.NoError(t, checker.CheckSessionEpoch(ctx, account, 11))
	floor = 0
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 11), ErrDependencyUnavailable)
	remoteErr = status.Error(codes.Unavailable, "offline")
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 11), ErrDependencyUnavailable)
	remoteErr = status.Error(codes.NotFound, "unknown/deleted")
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 11), ErrInitiator)
	require.Equal(t, 6, calls)
	require.Error(t, checker.CheckSessionEpoch(ctx, "bad-account", 11))
	require.Error(t, checker.CheckSessionEpoch(ctx, account, 0))
	require.Equal(t, 6, calls, "invalid input never calls Auth")
	require.ErrorIs(t, (*AuthFloorChecker)(nil).CheckSessionEpoch(ctx, account, 11), ErrDependencyUnavailable)
	checker.Client = floorClientFunc(func(context.Context, *authv1.GetChatSessionEpochFloorRequest) (*authv1.GetChatSessionEpochFloorResponse, error) {
		return nil, nil
	})
	require.ErrorIs(t, checker.CheckSessionEpoch(ctx, account, 11), ErrDependencyUnavailable)
}
