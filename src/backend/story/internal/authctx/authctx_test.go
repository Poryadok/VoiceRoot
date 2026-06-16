package authctx_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/story/internal/authctx"
)

func TestProfileID_missing(t *testing.T) {
	_, err := authctx.ProfileID(context.Background())
	require.Error(t, err)
}

func TestProfileID_ok(t *testing.T) {
	id := uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(authctx.HeaderProfileID, id.String()))
	got, err := authctx.ProfileID(ctx)
	require.NoError(t, err)
	require.Equal(t, id, got)
}
