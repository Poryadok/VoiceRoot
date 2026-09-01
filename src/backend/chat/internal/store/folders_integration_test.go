package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func applyFolderMigrationsForStoreTest(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	applyChatMigrationFile(t, ctx, pool, "000008_folders.up.sql")
	applyChatMigrationFile(t, ctx, pool, "000009_folder_chats.up.sql")
}

// TestListFolders_SeedsSystemFolders documents navigation.md § default folders + lazy init.
func TestListFolders_SeedsSystemFolders(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyFolderMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	profileID := uuid.New()
	folders, err := store.ListFolders(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, folders, 5)
	require.Equal(t, "All", folders[0].Name)
	require.Equal(t, "system", folders[0].FolderType)
	require.Equal(t, "DM", folders[1].Name)
	require.Equal(t, "Groups", folders[2].Name)
	require.Equal(t, "Channels", folders[3].Name)
	require.Equal(t, "Spaces", folders[4].Name)

	// Idempotent: second list does not duplicate system folders.
	again, err := store.ListFolders(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, again, 5)
}

// TestCreateFolder_CustomAppends documents custom folder CRUD foundation slice.
func TestCreateFolder_CustomAppends(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyFolderMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	profileID := uuid.New()
	_, err := store.ListFolders(ctx, profileID)
	require.NoError(t, err)

	custom, err := store.CreateFolder(ctx, profileID, "Work", `{"chat_ids":["a"]}`)
	require.NoError(t, err)
	require.Equal(t, "custom", custom.FolderType)
	require.Equal(t, "Work", custom.Name)
	require.Equal(t, int32(5), custom.SortOrder)

	_, err = store.CreateFolder(ctx, profileID, "  Personal  ", "")
	require.NoError(t, err)

	all, err := store.ListFolders(ctx, profileID)
	require.NoError(t, err)
	require.Len(t, all, 7)
	require.Equal(t, "Work", all[5].Name)
	require.Equal(t, "Personal", all[6].Name)
	require.Equal(t, int32(6), all[6].SortOrder)

	_, err = store.CreateFolder(ctx, profileID, "   ", "")
	require.Error(t, err)
}
