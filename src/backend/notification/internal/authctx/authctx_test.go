package authctx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/notification/internal/authctx"
)

func TestProfileID_MissingMetadata(t *testing.T) {
	t.Parallel()

	_, ok := authctx.ProfileID(context.Background())
	require.False(t, ok)
}

func TestProfileID_Valid(t *testing.T) {
	t.Parallel()

	want := uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, want.String(),
	))
	got, ok := authctx.ProfileID(ctx)
	require.True(t, ok)
	require.Equal(t, want, got)
}

func TestProfileID_EmptyValue(t *testing.T) {
	t.Parallel()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, "",
	))
	_, ok := authctx.ProfileID(ctx)
	require.False(t, ok)
}

func TestProfileID_InvalidUUID(t *testing.T) {
	t.Parallel()

	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		authctx.HeaderProfileID, "not-a-uuid",
	))
	_, ok := authctx.ProfileID(ctx)
	require.False(t, ok)
}
