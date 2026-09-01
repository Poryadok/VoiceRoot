package store

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedMembershipChannel(t *testing.T, ctx context.Context, s *DMStore, owner uuid.UUID, name string) uuid.UUID {
	t.Helper()
	var chatID uuid.UUID
	err := s.Pool.QueryRow(ctx, `
INSERT INTO chats (type, name, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed)
VALUES ('channel', $1, $2, 0, true, false)
RETURNING id
`, name, owner).Scan(&chatID)
	require.NoError(t, err)
	_, err = s.Pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket)
VALUES ($1, $2, 'owner', 'main')
`, chatID, owner)
	require.NoError(t, err)
	return chatID
}

// TestListChatsPage_includesMembershipChannel documents navigation.md: membership channels
// must appear in main inbox alongside dm/group rows (not only via space merge).
func TestListChatsPage_includesMembershipChannel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	channelID := seedMembershipChannel(t, ctx, s, owner, "Announcements")
	_, _, err := s.EnsureDM(ctx, owner, uuid.New(), InboxMain)
	require.NoError(t, err)

	at := time.Now().UTC().Add(-time.Minute)
	_, err = pool.Exec(ctx, `UPDATE chats SET last_message_at = $2 WHERE id = $1`, channelID, at)
	require.NoError(t, err)

	page, err := s.ListChatsPage(ctx, owner, "", 10, "main", nil)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(page.Rows), 2)

	var found bool
	for _, row := range page.Rows {
		if row.ID == channelID {
			found = true
			require.Equal(t, "channel", row.Type)
			require.NotNil(t, row.Name)
			require.Equal(t, "Announcements", *row.Name)
			require.True(t, row.ThreadsEnabled)
			require.False(t, row.AllowUserMainFeed)
		}
	}
	require.True(t, found, "membership channel must appear in ListChatsPage main inbox")
}

// TestListChatsPage_excludesArchivedMembershipChannel documents archived channels stay hidden.
func TestListChatsPage_excludesArchivedMembershipChannel(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	channelID := seedMembershipChannel(t, ctx, s, owner, "Archived channel")
	_, err := pool.Exec(ctx, `
UPDATE chat_members SET is_archived = true WHERE chat_id = $1 AND profile_id = $2
`, channelID, owner)
	require.NoError(t, err)

	page, err := s.ListChatsPage(ctx, owner, "", 10, "main", nil)
	require.NoError(t, err)
	for _, row := range page.Rows {
		require.NotEqual(t, channelID, row.ID, "archived channel must not appear in main inbox")
	}
}

// TestListChatsPage_archiveInbox lists only archived membership chats (text-chat.md §Архивирование).
func TestListChatsPage_archiveInbox(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	owner := uuid.New()
	activeID := seedMembershipChannel(t, ctx, s, owner, "Active channel")
	archivedID := seedMembershipChannel(t, ctx, s, owner, "Archived channel")
	_, err := pool.Exec(ctx, `
UPDATE chat_members SET is_archived = true WHERE chat_id = $1 AND profile_id = $2
`, archivedID, owner)
	require.NoError(t, err)

	page, err := s.ListChatsPage(ctx, owner, "", 10, "archive", nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	require.Equal(t, archivedID, page.Rows[0].ID)
	require.NotEqual(t, activeID, page.Rows[0].ID)
}

// TestListChatsPage_mainWithSpacesPagination documents R3-A16: space chats paginate with membership rows.
func TestListChatsPage_mainWithSpacesPagination(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	s := &DMStore{Pool: pool}

	viewer := uuid.New()
	spaceID := uuid.New()
	other := uuid.New()

	dmNewer, _, err := s.EnsureDM(ctx, viewer, other, InboxMain)
	require.NoError(t, err)

	var spaceChannelID uuid.UUID
	err = pool.QueryRow(ctx, `
INSERT INTO chats (type, space_id, name, creator_profile_id, slow_mode_seconds)
VALUES ('channel', $1, 'general', $2, 0)
RETURNING id
`, spaceID, viewer).Scan(&spaceChannelID)
	require.NoError(t, err)

	now := time.Now().UTC()
	_, err = pool.Exec(ctx, `UPDATE chats SET last_message_at = $2 WHERE id = $1`, dmNewer.ID, now.Add(2*time.Minute))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE chats SET last_message_at = $2 WHERE id = $1`, spaceChannelID, now.Add(time.Minute))
	require.NoError(t, err)

	page1, err := s.ListChatsPage(ctx, viewer, "", 1, "main", []uuid.UUID{spaceID})
	require.NoError(t, err)
	require.Len(t, page1.Rows, 1)
	require.Equal(t, dmNewer.ID, page1.Rows[0].ID)
	require.NotEmpty(t, page1.NextCursor)

	page2, err := s.ListChatsPage(ctx, viewer, page1.NextCursor, 1, "main", []uuid.UUID{spaceID})
	require.NoError(t, err)
	require.Len(t, page2.Rows, 1)
	require.Equal(t, spaceChannelID, page2.Rows[0].ID)
}
