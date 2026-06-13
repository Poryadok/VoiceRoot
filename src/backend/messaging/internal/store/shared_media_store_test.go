package store

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestSharedMediaStore_listAttachmentsAndLinks(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000008_shared_media_indexes.up.sql"))

	chatID := uuid.New()
	sender := uuid.New()
	msgImg := uuid.New()
	msgLink := uuid.New()
	fileID := uuid.New()

	store := &SharedMediaStore{Pool: pool}
	require.NoError(t, InsertMessageAttachments(ctx, pool, msgImg, chatID, sender, []map[string]string{
		{"file_id": fileID.String(), "type": "image"},
	}, " "))
	_, err := pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'dm', $3, $4, '[]'::jsonb, '[]'::jsonb)
`, msgLink, chatID, sender, "link https://example.com")
	require.NoError(t, err)

	media, _, hasMore, err := store.List(ctx, chatID, SharedMediaKindMedia, "", 20)
	require.NoError(t, err)
	require.False(t, hasMore)
	require.Len(t, media, 1)
	require.Equal(t, fileID, *media[0].FileID)

	links, _, _, err := store.List(ctx, chatID, SharedMediaKindLinks, "", 20)
	require.NoError(t, err)
	require.Len(t, links, 1)
	require.Equal(t, "https://example.com", links[0].ExternalURL)
}
