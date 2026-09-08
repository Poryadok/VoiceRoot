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
