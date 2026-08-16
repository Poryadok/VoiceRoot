package authctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestProfileID_FromMetadata(t *testing.T) {
	t.Parallel()
	id := uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderProfileID, id.String(),
	))
	got, ok := ProfileID(ctx)
	require.True(t, ok)
	require.Equal(t, id, got)
}

func TestIsStaff_FromRolesMetadata(t *testing.T) {
	t.Parallel()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderRoles, "member,staff",
	))
	require.True(t, IsStaff(ctx))
	require.False(t, IsStaff(context.Background()))
}

