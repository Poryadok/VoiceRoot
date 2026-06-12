package authctx

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestProfileIDAndAccountID_fromIncomingMetadata(t *testing.T) {
	t.Parallel()
	profileID := uuid.New()
	accountID := uuid.New()
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		HeaderProfileID, profileID.String(),
		HeaderAccountID, accountID.String(),
	))
	gotProfile, ok := ProfileID(ctx)
	require.True(t, ok)
	require.Equal(t, profileID, gotProfile)
	gotAccount, ok := AccountID(ctx)
	require.True(t, ok)
	require.Equal(t, accountID, gotAccount)
}
