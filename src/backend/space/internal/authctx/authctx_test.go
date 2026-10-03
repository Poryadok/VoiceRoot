package authctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAccountID(t *testing.T) {
	t.Parallel()
	valid := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

	for _, tc := range []struct {
		name string
		ctx  context.Context
		ok   bool
	}{
		{name: "missing metadata", ctx: context.Background()},
		{name: "empty header", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderUserID, ""))},
		{name: "invalid uuid", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderUserID, "not-a-uuid"))},
		{name: "valid", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderUserID, valid.String())), ok: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := AccountID(tc.ctx)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, valid, got)
			}
		})
	}
}

func TestProfileID(t *testing.T) {
	t.Parallel()
	valid := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	for _, tc := range []struct {
		name string
		ctx  context.Context
		ok   bool
	}{
		{name: "missing metadata", ctx: context.Background()},
		{name: "empty header", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderProfileID, ""))},
		{name: "invalid uuid", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderProfileID, "bad"))},
		{name: "valid", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderProfileID, valid.String())), ok: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ProfileID(tc.ctx)
			require.Equal(t, tc.ok, ok)
			if tc.ok {
				require.Equal(t, valid, got)
			}
		})
	}
}

func TestSessionEpoch(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want int64
		ok   bool
	}{
		{name: "missing metadata", ctx: context.Background()},
		{name: "missing header", ctx: metadata.NewIncomingContext(context.Background(), metadata.MD{})},
		{name: "empty", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderSessionEpoch, ""))},
		{name: "not integer", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderSessionEpoch, "one"))},
		{name: "zero", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderSessionEpoch, "0"))},
		{name: "negative", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderSessionEpoch, "-1"))},
		{name: "multiple", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderSessionEpoch, "7", HeaderSessionEpoch, "8"))},
		{name: "valid", ctx: metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderSessionEpoch, "7")), want: 7, ok: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := SessionEpoch(tc.ctx)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestVerifiedServiceIdentityUnaryInterceptor_BlankTokenFailsClosed(t *testing.T) {
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer valid-looking-token"))
	for _, configuredToken := range []string{"", " \t "} {
		handlerCalled := false
		_, err := VerifiedServiceIdentityUnaryInterceptor(configuredToken)(ctx, nil, &grpc.UnaryServerInfo{
			FullMethod: "/voice.space.v1.SpaceService/ResolveVoiceRoomAccess",
		}, func(context.Context, any) (any, error) {
			handlerCalled = true
			return nil, nil
		})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
		require.False(t, handlerCalled)
	}
}

func TestVerifiedGameIntegrationIdentityIsRPCScopedAndFailsClosed(t *testing.T) {
	const method = "/voice.space.v1.SpaceService/CreateCommunityBootstrap"
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer gis-secret"))
	handlerCalled := false
	_, err := VerifiedGameIntegrationUnaryInterceptor("")(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(context.Context, any) (any, error) {
		handlerCalled = true
		return nil, nil
	})
	require.Equal(t, codes.Unauthenticated, status.Code(err))
	require.False(t, handlerCalled)

	var identity ServiceIdentity
	_, err = VerifiedGameIntegrationUnaryInterceptor("gis-secret")(ctx, nil, &grpc.UnaryServerInfo{FullMethod: method}, func(ctx context.Context, _ any) (any, error) {
		identity, _ = VerifiedServiceIdentity(ctx)
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, ServiceIdentityGameIntegration, identity)
	identity = ""
	_, err = VerifiedGameIntegrationUnaryInterceptor("gis-secret")(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/voice.space.v1.SpaceService/RecoverCommunityOwner"}, func(ctx context.Context, _ any) (any, error) {
		identity, _ = VerifiedServiceIdentity(ctx)
		return nil, nil
	})
	require.NoError(t, err)
	require.Equal(t, ServiceIdentityGameIntegration, identity)

	identity = ""
	_, err = VerifiedGameIntegrationUnaryInterceptor("gis-secret")(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/voice.space.v1.SpaceService/GetSpace"}, func(ctx context.Context, _ any) (any, error) {
		identity, _ = VerifiedServiceIdentity(ctx)
		return nil, nil
	})
	require.NoError(t, err)
	require.Empty(t, identity, "GIS bearer only grants the dedicated bootstrap RPC")
}
