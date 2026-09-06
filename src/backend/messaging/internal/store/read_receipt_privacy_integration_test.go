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
	seedChatSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	chatID, profileID, peerID := uuid.New(), uuid.New(), uuid.New()
	messageID, peerMessageID := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	_, err := pool.Exec(ctx, `INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds) VALUES ($1, 'dm', $2, 0)`, chatID, profileID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member'), ($1, $3, 'member')`, chatID, profileID, peerID)
	require.NoError(t, err)
	require.NoError(t, store.UpsertReadReceipt(ctx, chatID, profileID, messageID))
	require.NoError(t, store.UpsertReadReceipt(ctx, chatID, peerID, peerMessageID))
	revoked, err := store.ClearPublicReadReceiptsForProfile(ctx, profileID, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.ElementsMatch(t, []PublicReadReceipt{
		{ChatID: chatID, ProfileID: profileID, MessageID: messageID, RecipientProfileID: profileID},
		{ChatID: chatID, ProfileID: peerID, MessageID: peerMessageID, RecipientProfileID: profileID},
	}, revoked)
	public, _, err := store.GetReadReceipt(ctx, chatID, profileID)
	require.NoError(t, err)
	require.Nil(t, public)
	peerPublic, _, err := store.GetReadReceipt(ctx, chatID, peerID)
	require.NoError(t, err)
	require.Equal(t, peerMessageID, *peerPublic)
}

func TestClearPublicReadReceiptsForProfileRevokesPeerWithoutOwnReceipt(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	chatID, optedOut, peer, messageID := uuid.New(), uuid.New(), uuid.New(), uuid.Must(uuid.NewV7())
	require.NoError(t, store.UpsertReadReceipt(ctx, chatID, peer, messageID))
	revoked, err := store.ClearPublicReadReceiptsForProfile(ctx, optedOut, []uuid.UUID{chatID})
	require.NoError(t, err)
	require.Equal(t, []PublicReadReceipt{{ChatID: chatID, ProfileID: peer, MessageID: messageID, RecipientProfileID: optedOut}}, revoked)
	public, _, err := store.GetReadReceipt(ctx, chatID, peer)
	require.NoError(t, err)
	require.Equal(t, messageID, *public)
}
