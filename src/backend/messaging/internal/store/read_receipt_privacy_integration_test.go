package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestClearPublicReadReceiptsForProfileReturnsPreUpdateCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	chatID, profileID, messageID := uuid.New(), uuid.New(), uuid.Must(uuid.NewV7())
	require.NoError(t, store.UpsertReadReceipt(ctx, chatID, profileID, messageID))
	revoked, err := store.ClearPublicReadReceiptsForProfile(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, []PublicReadReceipt{{ChatID: chatID, ProfileID: profileID, MessageID: messageID}}, revoked)
	public, _, err := store.GetReadReceipt(ctx, chatID, profileID)
	require.NoError(t, err)
	require.Nil(t, public)
}
