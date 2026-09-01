package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// TestFolderMembership_CustomAddListReorderPin documents chat-service.md § folder membership + pin.
func TestFolderMembership_CustomAddListReorderPin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyFolderMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	profileID := uuid.New()
	peerA := uuid.New()
	peerB := uuid.New()
	dmA, _, err := store.EnsureDM(ctx, profileID, peerA, InboxMain)
	require.NoError(t, err)
	dmB, _, err := store.EnsureDM(ctx, profileID, peerB, InboxMain)
	require.NoError(t, err)

	_, err = store.ListFolders(ctx, profileID)
	require.NoError(t, err)
	custom, err := store.CreateFolder(ctx, profileID, "Work", `{}`)
	require.NoError(t, err)

	err = store.AddChatToFolder(ctx, profileID, custom.ID, dmA.ID, nil)
	require.NoError(t, err)
	err = store.AddChatToFolder(ctx, profileID, custom.ID, dmB.ID, nil)
	require.NoError(t, err)

	page, err := store.ListChatsPageByFolder(ctx, profileID, custom.ID, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 2)

	err = store.ReorderFolderChats(ctx, profileID, custom.ID, []uuid.UUID{dmB.ID, dmA.ID})
	require.NoError(t, err)
	page, err = store.ListChatsPageByFolder(ctx, profileID, custom.ID, "", 10, nil)
	require.NoError(t, err)
	require.Equal(t, dmB.ID, page.Rows[0].ID)

	err = store.PinChatInFolder(ctx, profileID, custom.ID, dmA.ID, nil)
	require.NoError(t, err)
	page, err = store.ListChatsPageByFolder(ctx, profileID, custom.ID, "", 10, nil)
	require.NoError(t, err)
	require.Equal(t, dmA.ID, page.Rows[0].ID, "pinned chat sorts first")

	err = store.UnpinChatInFolder(ctx, profileID, custom.ID, dmA.ID)
	require.NoError(t, err)

	err = store.RemoveChatFromFolder(ctx, profileID, custom.ID, dmB.ID)
	require.NoError(t, err)
	page, err = store.ListChatsPageByFolder(ctx, profileID, custom.ID, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
}

// TestFolderMembership_SystemFolderFilterAndPinOverlay documents system folder predicate + pin overlay.
func TestFolderMembership_SystemFolderFilterAndPinOverlay(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyFolderMigrationsForStoreTest(t, ctx, pool)
	store := &DMStore{Pool: pool}

	profileID := uuid.New()
	peer := uuid.New()
	dm, _, err := store.EnsureDM(ctx, profileID, peer, InboxMain)
	require.NoError(t, err)
	group, err := store.CreateGroupChat(ctx, profileID, "g")
	require.NoError(t, err)

	folders, err := store.ListFolders(ctx, profileID)
	require.NoError(t, err)
	var dmFolderID uuid.UUID
	for _, f := range folders {
		if f.Name == "DM" {
			dmFolderID = f.ID
			break
		}
	}
	require.NotEqual(t, uuid.Nil, dmFolderID)

	page, err := store.ListChatsPageByFolder(ctx, profileID, dmFolderID, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
	require.Equal(t, dm.ID, page.Rows[0].ID)

	err = store.AddChatToFolder(ctx, profileID, dmFolderID, dm.ID, nil)
	require.ErrorIs(t, err, ErrSystemFolderMembership)

	err = store.PinChatInFolder(ctx, profileID, dmFolderID, group.ID, nil)
	require.ErrorIs(t, err, ErrFolderChatPredicate)

	err = store.PinChatInFolder(ctx, profileID, dmFolderID, dm.ID, nil)
	require.NoError(t, err)
	page, err = store.ListChatsPageByFolder(ctx, profileID, dmFolderID, "", 10, nil)
	require.NoError(t, err)
	require.Len(t, page.Rows, 1)
}
