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
		row, _, err := store.EnsureDM(ctx, profileID, peer)
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

func applyChatMigrationFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	root := chatRepoRoot(t)
	sqlBytes, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "chat_db", name))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}
