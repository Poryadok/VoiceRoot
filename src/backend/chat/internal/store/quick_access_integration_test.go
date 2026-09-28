package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// TestQuickAccess_AddListReorderLimit documents chat-service.md § Quick Access.
func TestQuickAccess_AddListReorderLimit(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000010_quick_access_chats.up.sql")
	store := &DMStore{Pool: pool}

	profileID := uuid.New()
	chatIDs := make([]uuid.UUID, 0, 16)
	for i := 0; i < 16; i++ {
		peer := uuid.New()
		row, _, err := store.EnsureDM(ctx, profileID, peer, InboxMain)
		require.NoError(t, err)
		chatIDs = append(chatIDs, row.ID)
	}

	for i := 0; i < 15; i++ {
		err := store.AddQuickAccess(ctx, profileID, chatIDs[i], nil)
		require.NoError(t, err)
	}

	err := store.AddQuickAccess(ctx, profileID, chatIDs[15], nil)
	require.ErrorIs(t, err, ErrQuickAccessLimit)

	list, err := store.ListQuickAccess(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, list, 15)

	err = store.AddQuickAccess(ctx, profileID, chatIDs[0], nil)
	require.NoError(t, err)

	reordered := []uuid.UUID{chatIDs[2], chatIDs[0], chatIDs[1]}
	for i := 3; i < 15; i++ {
		reordered = append(reordered, chatIDs[i])
	}
	err = store.ReorderQuickAccess(ctx, profileID, reordered)
	require.NoError(t, err)

	list, err = store.ListQuickAccess(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, chatIDs[2], list[0].ChatID)
	require.Equal(t, chatIDs[0], list[1].ChatID)

	err = store.RemoveQuickAccess(ctx, profileID, chatIDs[2])
	require.NoError(t, err)
	list, err = store.ListQuickAccess(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, list, 14)
	for i, row := range list {
		require.Equal(t, int32(i), row.SortOrder)
	}
}

func TestQuickAccess_RequiresMembership(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000010_quick_access_chats.up.sql")
	store := &DMStore{Pool: pool}

	err := store.AddQuickAccess(ctx, uuid.New(), uuid.New(), nil)
	require.ErrorIs(t, err, pgx.ErrNoRows)
}

func TestQuickAccess_ReplaceAtLimitIsAtomic(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000010_quick_access_chats.up.sql")
	store := &DMStore{Pool: pool}

	profileID := uuid.New()
	chatIDs := make([]uuid.UUID, 0, 17)
	for i := 0; i < 17; i++ {
		row, _, err := store.EnsureDM(ctx, profileID, uuid.New(), InboxMain)
		require.NoError(t, err)
		chatIDs = append(chatIDs, row.ID)
	}
	for _, chatID := range chatIDs[:15] {
		require.NoError(t, store.AddQuickAccess(ctx, profileID, chatID, nil))
	}
	const replacedSlot = 7
	oldChatID := chatIDs[replacedSlot]
	newChatID := chatIDs[15]
	archivedChatID := chatIDs[16]
	require.NoError(t, store.SetMemberArchived(ctx, archivedChatID, profileID, true))
	otherProfileID := uuid.New()
	otherChat, _, err := store.EnsureDM(ctx, otherProfileID, uuid.New(), InboxMain)
	require.NoError(t, err)

	before, err := store.ListQuickAccess(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, before, 15)

	for _, tc := range []struct {
		name      string
		newChatID uuid.UUID
	}{
		{name: "unknown chat", newChatID: uuid.New()},
		{name: "another profile membership", newChatID: otherChat.ID},
		{name: "archived membership", newChatID: archivedChatID},
		{name: "already present", newChatID: chatIDs[0]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := store.ReplaceQuickAccess(ctx, profileID, tc.newChatID, oldChatID)
			require.Error(t, err)
			after, err := store.ListQuickAccess(ctx, profileID)
			require.NoError(t, err)
			require.Equal(t, before, after, "failed replacement must preserve all 15 slots and their order")
		})
	}
	require.ErrorIs(t, store.ReplaceQuickAccess(ctx, profileID, newChatID, uuid.New()), ErrQuickAccessReplaceSlotNotFound)
	staleAfter, err := store.ListQuickAccess(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, before, staleAfter)

	require.NoError(t, store.ReplaceQuickAccess(ctx, profileID, newChatID, oldChatID))
	after, err := store.ListQuickAccess(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, after, 15)
	for i, row := range after {
		wantChatID := before[i].ChatID
		if i == replacedSlot {
			wantChatID = newChatID
		}
		require.Equal(t, wantChatID, row.ChatID, "slot %d", i)
		require.Equal(t, before[i].SortOrder, row.SortOrder, "slot %d", i)
	}
}

func applyChatMigrationFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	root := chatRepoRoot(t)
	sqlBytes, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "chat_db", name))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}
