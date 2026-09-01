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

// TestUpdateFolder_CustomRenames documents navigation.md § custom folder rename.
func TestUpdateFolder_CustomRenames(t *testing.T) {
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

	custom, err := store.CreateFolder(ctx, profileID, "Work", `{}`)
	require.NoError(t, err)

	newName := "Projects"
	updated, err := store.UpdateFolder(ctx, profileID, custom.ID, FolderUpdate{Name: &newName})
	require.NoError(t, err)
	require.Equal(t, "Projects", updated.Name)

	newOrder := int32(99)
	updated, err = store.UpdateFolder(ctx, profileID, custom.ID, FolderUpdate{SortOrder: &newOrder})
	require.NoError(t, err)
	require.Equal(t, int32(99), updated.SortOrder)
}

// TestUpdateFolder_SystemFolderRejected documents system folders are immutable.
func TestUpdateFolder_SystemFolderRejected(t *testing.T) {
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
	require.NotEmpty(t, folders)

	newName := "Nope"
	_, err = store.UpdateFolder(ctx, profileID, folders[0].ID, FolderUpdate{Name: &newName})
	require.ErrorIs(t, err, ErrSystemFolderImmutable)
}

// TestDeleteFolder_CustomRemovesRow documents custom folder delete + cascade membership.
func TestDeleteFolder_CustomRemovesRow(t *testing.T) {
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

	custom, err := store.CreateFolder(ctx, profileID, "Temp", `{}`)
	require.NoError(t, err)

	dm, _, err := store.EnsureDM(ctx, profileID, uuid.New())
	require.NoError(t, err)
	require.NoError(t, store.AddChatToFolder(ctx, profileID, custom.ID, dm.ID, nil))

	require.NoError(t, store.DeleteFolder(ctx, profileID, custom.ID))

	all, err := store.ListFolders(ctx, profileID)
	require.NoError(t, err)
	for _, f := range all {
		require.NotEqual(t, custom.ID, f.ID)
	}

	var fcCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM folder_chats WHERE folder_id = $1`, custom.ID).Scan(&fcCount))
	require.Zero(t, fcCount)
}

// TestDeleteFolder_SystemFolderRejected documents system folders cannot be deleted.
func TestDeleteFolder_SystemFolderRejected(t *testing.T) {
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

	err = store.DeleteFolder(ctx, profileID, folders[0].ID)
	require.ErrorIs(t, err, ErrSystemFolderImmutable)
}
