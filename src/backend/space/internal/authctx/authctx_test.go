package authctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestAccountID(t *testing.T) {
	t.Parallel()
	valid := uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

	t.Run("missing metadata", func(t *testing.T) {
		t.Parallel()
		_, ok := AccountID(context.Background())
		require.False(t, ok)
	})

	t.Run("empty header", func(t *testing.T) {
		t.Parallel()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderUserID, ""))
		_, ok := AccountID(ctx)
		require.False(t, ok)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderUserID, "not-a-uuid"))
		_, ok := AccountID(ctx)
		require.False(t, ok)
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderUserID, valid.String()))
		got, ok := AccountID(ctx)
		require.True(t, ok)
		require.Equal(t, valid, got)
	})
}

func TestProfileID(t *testing.T) {
	t.Parallel()
	valid := uuid.MustParse("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb")

	t.Run("missing metadata", func(t *testing.T) {
		t.Parallel()
		_, ok := ProfileID(context.Background())
		require.False(t, ok)
	})

	t.Run("empty header", func(t *testing.T) {
		t.Parallel()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderProfileID, ""))
		_, ok := ProfileID(ctx)
		require.False(t, ok)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		t.Parallel()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderProfileID, "bad"))
		_, ok := ProfileID(ctx)
		require.False(t, ok)
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(HeaderProfileID, valid.String()))
		got, ok := ProfileID(ctx)
		require.True(t, ok)
		require.Equal(t, valid, got)
	})
}
