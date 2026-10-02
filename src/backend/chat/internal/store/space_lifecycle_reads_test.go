package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

func TestSpaceLifecycleHidesOrdinaryChatAndFolderReadsUntilRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL")
	}
	ctx := context.Background()
	pool := startChatDBForStoreTest(t, ctx)
	applyChatMigrationsForStoreTest(t, ctx, pool)
	applyChatMigrationFile(t, ctx, pool, "000008_folders.up.sql")
	applyChatMigrationFile(t, ctx, pool, "000009_folder_chats.up.sql")
	applyChatMigrationFile(t, ctx, pool, "000010_quick_access_chats.up.sql")
	dm := &DMStore{Pool: pool}
	lifecycle := &SpaceLifecycleStore{Pool: pool}
	viewer, space, controlSpace := uuid.New(), uuid.New(), uuid.New()
	target, err := dm.CreateSpaceGroupChat(ctx, viewer, space, "private frozen chat", nil)
	require.NoError(t, err)
	control, err := dm.CreateSpaceGroupChat(ctx, viewer, controlSpace, "live control", nil)
	require.NoError(t, err)
	for _, chat := range []*ChatRow{target, control} {
		_, err = pool.Exec(ctx, `INSERT INTO chat_members(chat_id,profile_id,role,inbox_bucket) VALUES($1,$2,'owner','main')`, chat.ID, viewer)
		require.NoError(t, err)
	}
	standalone, err := dm.CreateGroupChat(ctx, viewer, "standalone control", nil)
	require.NoError(t, err)
	folders, err := dm.ListFolders(ctx, viewer)
	require.NoError(t, err)
	custom, err := dm.CreateFolder(ctx, viewer, "custom", "")
	require.NoError(t, err)
	for _, chat := range []*ChatRow{target, control, standalone} {
		require.NoError(t, dm.AddChatToFolder(ctx, viewer, custom.ID, chat.ID, nil))
		require.NoError(t, dm.AddQuickAccess(ctx, viewer, chat.ID, nil))
	}
	folders = append(folders, *custom)
	spaces := []uuid.UUID{space, controlSpace}
	check := func(hidden bool) {
		t.Helper()
		row, err := dm.FindChatByID(ctx, target.ID)
		require.NoError(t, err)
		if hidden {
			require.Nil(t, row, "ordinary resource lookup cannot expose frozen metadata")
		} else {
			require.NotNil(t, row)
		}
		for _, scopedSpaces := range [][]uuid.UUID{nil, spaces} {
			page, err := dm.ListChatsPage(ctx, viewer, "", 100, "main", scopedSpaces)
			require.NoError(t, err)
			assertLifecycleChatRows(t, page.Rows, target.ID, control.ID, standalone.ID, hidden)
			for _, folder := range folders {
				page, err = dm.ListChatsPageByFolder(ctx, viewer, folder.ID, "", 100, scopedSpaces)
				require.NoError(t, err)
				if hidden {
					for _, row := range page.Rows {
						require.NotEqual(t, target.ID, row.ID, "folder=%s cannot leak frozen chat", folder.Name)
					}
				}
				if folder.Name == "All" || folder.ID == custom.ID {
					assertLifecycleChatRows(t, page.Rows, target.ID, control.ID, standalone.ID, hidden)
				}
			}
		}
		rows, err := dm.ListSpaceChatsForProfile(ctx, viewer, spaces)
		require.NoError(t, err)
		var ids []uuid.UUID
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		require.Contains(t, ids, control.ID)
		if hidden {
			require.NotContains(t, ids, target.ID)
		} else {
			require.Contains(t, ids, target.ID)
		}
		row, err = dm.FindChatByID(ctx, control.ID)
		require.NoError(t, err)
		require.NotNil(t, row)
		quick, err := dm.ListQuickAccess(ctx, viewer)
		require.NoError(t, err)
		var quickIDs []uuid.UUID
		for _, slot := range quick {
			quickIDs = append(quickIDs, slot.ChatID)
		}
		require.Contains(t, quickIDs, control.ID)
		require.Contains(t, quickIDs, standalone.ID)
		if hidden {
			require.NotContains(t, quickIDs, target.ID, "a frozen slot must not make every hydrated shortcut unavailable")
		} else {
			require.Contains(t, quickIDs, target.ID, "restore preserves the exact shortcut")
		}
	}
	check(false)
	operation := uuid.New()
	prepared, err := lifecycle.PrepareSpaceDeletionManifest(ctx, &chatv1.PrepareSpaceDeletionManifestRequest{
		ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), ScheduleGeneration: 1,
	})
	require.NoError(t, err)
	check(true)
	_, err = lifecycle.ApplySpaceLifecycleFence(ctx, &chatv1.ApplySpaceLifecycleFenceRequest{Fence: &commonv1.SpaceLifecycleFenceRequest{
		ProtocolVersion: 1, SpaceId: space.String(), DeletionOperationId: operation.String(), Generation: 2,
		DesiredState: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_LIVE, Manifest: prepared.GetReceipt().GetChatManifest(),
	}})
	require.NoError(t, err)
	check(false)
}

func assertLifecycleChatRows(t *testing.T, rows []*ChatRow, target, control, standalone uuid.UUID, hidden bool) {
	t.Helper()
	var ids []uuid.UUID
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	require.Contains(t, ids, control)
	require.Contains(t, ids, standalone)
	if hidden {
		require.NotContains(t, ids, target)
	} else {
		require.Contains(t, ids, target)
	}
}
