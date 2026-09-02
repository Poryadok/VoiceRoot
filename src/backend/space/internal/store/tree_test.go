package store

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestTree_CreateCategory_ListOrdered(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Tree Space", "", "private")
	require.NoError(t, err)

	catB, err := st.CreateCategory(ctx, space.ID, "B", 1)
	require.NoError(t, err)
	catA, err := st.CreateCategory(ctx, space.ID, "A", 0)
	require.NoError(t, err)

	cats, err := st.ListCategories(ctx, space.ID)
	require.NoError(t, err)
	require.Len(t, cats, 2)
	require.Equal(t, catA.ID, cats[0].ID)
	require.Equal(t, catB.ID, cats[1].ID)
}

func TestTree_CreateVoiceRoom_AutoTreeNode(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "VR Space", "", "private")
	require.NoError(t, err)

	room, node, err := st.CreateVoiceRoom(ctx, space.ID, "Lobby", nil)
	require.NoError(t, err)
	require.Equal(t, TreeKindVoiceRoom, node.Kind)
	require.NotNil(t, node.VoiceRoomID)
	require.Equal(t, room.ID, *node.VoiceRoomID)
	require.Equal(t, int32(0), node.SortOrder)
}

func TestTree_MixedNodes_UnifiedSortOrder(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Mixed", "", "private")
	require.NoError(t, err)

	chatID := uuid.New()
	textNode, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID,
		Kind:    TreeKindTextChat,
		ChatID:  &chatID,
	})
	require.NoError(t, err)

	_, voiceNode, err := st.CreateVoiceRoom(ctx, space.ID, "Voice", nil)
	require.NoError(t, err)

	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, textNode.ID, nodes[0].ID)
	require.Equal(t, voiceNode.ID, nodes[1].ID)
	require.Less(t, nodes[0].SortOrder, nodes[1].SortOrder)
}

func TestTree_ReorderSpaceTree(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Reorder", "", "private")
	require.NoError(t, err)

	chatA, chatB := uuid.New(), uuid.New()
	nodeA, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatA})
	require.NoError(t, err)
	_, nodeB, err := st.CreateVoiceRoom(ctx, space.ID, "V", nil)
	require.NoError(t, err)
	nodeC, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatB})
	require.NoError(t, err)

	err = st.ReorderSpaceTree(ctx, space.ID, []uuid.UUID{nodeC.ID, nodeA.ID, nodeB.ID})
	require.NoError(t, err)

	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Equal(t, nodeC.ID, nodes[0].ID)
	require.Equal(t, int32(0), nodes[0].SortOrder)
	require.Equal(t, nodeA.ID, nodes[1].ID)
	require.Equal(t, nodeB.ID, nodes[2].ID)
}

func TestTree_DeleteCategory_NullsNodeCategory(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "CatDel", "", "private")
	require.NoError(t, err)
	cat, err := st.CreateCategory(ctx, space.ID, "General", 0)
	require.NoError(t, err)

	chatID := uuid.New()
	_, err = st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatID, CategoryID: &cat.ID,
	})
	require.NoError(t, err)

	require.NoError(t, st.DeleteCategory(ctx, cat.ID))
	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Nil(t, nodes[0].CategoryID)
}

func TestTree_DeleteVoiceRoom_CascadesNode(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "VRDel", "", "private")
	require.NoError(t, err)
	room, _, err := st.CreateVoiceRoom(ctx, space.ID, "Tmp", nil)
	require.NoError(t, err)

	require.NoError(t, st.DeleteVoiceRoom(ctx, room.ID))
	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Empty(t, nodes)
}

func TestTree_NodeLimit(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Limit", "", "private")
	require.NoError(t, err)

	for i := 0; i < MaxTreeNodes; i++ {
		cid := uuid.New()
		_, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
			SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &cid,
		})
		require.NoError(t, err)
	}
	extra := uuid.New()
	_, err = st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &extra,
	})
	require.ErrorIs(t, err, ErrTreeNodeLimit)
}

func TestTree_ListSpaceTree_Aggregates(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Agg", "", "private")
	require.NoError(t, err)

	_, _, err = st.CreateVoiceRoom(ctx, space.ID, "Lobby", nil)
	require.NoError(t, err)

	data, err := st.ListSpaceTree(ctx, space.ID)
	require.NoError(t, err)
	require.Len(t, data.VoiceRooms, 1)
	require.Len(t, data.Nodes, 1)
}

func TestTree_RemoveTreeNode(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Rem", "", "private")
	require.NoError(t, err)
	chatID := uuid.New()
	node, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatID,
	})
	require.NoError(t, err)
	require.NoError(t, st.RemoveTreeNode(ctx, space.ID, node.ID))
	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Empty(t, nodes)
}

func TestTree_UpdateCategory_AndUpsertUpdate(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Upd", "", "private")
	require.NoError(t, err)
	cat, err := st.CreateCategory(ctx, space.ID, "Old", 0)
	require.NoError(t, err)

	newName := "New"
	updated, err := st.UpdateCategory(ctx, cat.ID, &newName, nil)
	require.NoError(t, err)
	require.Equal(t, "New", updated.Name)

	chatID := uuid.New()
	node, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatID,
	})
	require.NoError(t, err)

	sys := true
	updatedNode, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID, NodeID: &node.ID, Kind: TreeKindTextChat,
		ChatID: &chatID, CategoryID: &cat.ID, IsSystem: &sys,
	})
	require.NoError(t, err)
	require.True(t, updatedNode.IsSystem)
	require.NotNil(t, updatedNode.CategoryID)
}

func TestTree_PinUnpin_PinnedSortsAbove(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Pin", "", "private")
	require.NoError(t, err)

	chatA, chatB := uuid.New(), uuid.New()
	nodeA, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatA})
	require.NoError(t, err)
	nodeB, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatB})
	require.NoError(t, err)

	pinned, err := st.PinTreeNode(ctx, space.ID, nodeB.ID)
	require.NoError(t, err)
	require.True(t, pinned.IsPinned)
	require.NotNil(t, pinned.PinOrder)
	require.Equal(t, int32(0), *pinned.PinOrder)

	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	require.Equal(t, nodeB.ID, nodes[0].ID)
	require.True(t, nodes[0].IsPinned)
	require.Equal(t, nodeA.ID, nodes[1].ID)
	require.False(t, nodes[1].IsPinned)

	unpinned, err := st.UnpinTreeNode(ctx, space.ID, nodeB.ID)
	require.NoError(t, err)
	require.False(t, unpinned.IsPinned)
	require.Nil(t, unpinned.PinOrder)
}

func TestTree_ReorderSpaceTree_PinnedGroupUsesPinOrder(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "PinReorder", "", "private")
	require.NoError(t, err)

	chatA, chatB := uuid.New(), uuid.New()
	nodeA, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatA})
	require.NoError(t, err)
	nodeB, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatB})
	require.NoError(t, err)

	_, err = st.PinTreeNode(ctx, space.ID, nodeA.ID)
	require.NoError(t, err)
	_, err = st.PinTreeNode(ctx, space.ID, nodeB.ID)
	require.NoError(t, err)

	err = st.ReorderSpaceTree(ctx, space.ID, []uuid.UUID{nodeB.ID, nodeA.ID})
	require.NoError(t, err)

	nodes, err := st.ListTreeNodes(ctx, space.ID)
	require.NoError(t, err)
	require.Equal(t, nodeB.ID, nodes[0].ID)
	require.Equal(t, int32(0), *nodes[0].PinOrder)
	require.Equal(t, nodeA.ID, nodes[1].ID)
	require.Equal(t, int32(1), *nodes[1].PinOrder)
}

func TestTree_ReorderSpaceTree_MixedPinGroupRejected(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "MixedReorder", "", "private")
	require.NoError(t, err)

	chatA, chatB := uuid.New(), uuid.New()
	nodeA, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatA})
	require.NoError(t, err)
	nodeB, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chatB})
	require.NoError(t, err)

	_, err = st.PinTreeNode(ctx, space.ID, nodeA.ID)
	require.NoError(t, err)

	err = st.ReorderSpaceTree(ctx, space.ID, []uuid.UUID{nodeA.ID, nodeB.ID})
	require.ErrorIs(t, err, ErrInvalidReorder)
}

func TestTree_InvalidKind(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}

	owner := uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Bad", "", "private")
	require.NoError(t, err)

	chatID := uuid.New()
	_, err = st.UpsertTreeNode(ctx, UpsertTreeNodeInput{
		SpaceID: space.ID, Kind: "invalid", ChatID: &chatID,
	})
	require.ErrorIs(t, err, ErrInvalidTreeKind)
}
