package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestListSpaceTree_Member(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Tree"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	cat, err := client.CreateCategory(ctx, &spacev1.CreateCategoryRequest{
		SpaceId: spaceID, Name: "General", SortOrder: 0,
	})
	require.NoError(t, err)

	vr, err := client.CreateVoiceRoom(ctx, &spacev1.CreateVoiceRoomRequest{
		SpaceId: spaceID, Name: "Lobby",
	})
	require.NoError(t, err)

	chatID := uuid.New().String()
	chatType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId:    spaceID,
		Kind:       "text_chat",
		CategoryId: ptr(cat.GetCategory().GetId()),
		LinkedChat: &chatv1.ChatRef{Id: chatID, Type: &chatType},
	})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Len(t, tree.GetCategories(), 1)
	require.Len(t, tree.GetNodes(), 2)
	require.Len(t, tree.GetVoiceRooms(), 1)
	require.Equal(t, vr.GetVoiceRoom().GetId(), tree.GetVoiceRooms()[0].GetId())
}

func TestListSpaceTree_NonMemberDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	_, _, otherCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Private"})
	require.NoError(t, err)

	_, err = client.ListSpaceTree(otherCtx, &spacev1.ListSpaceTreeRequest{
		SpaceId: created.GetSpace().GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestReorderSpaceTree_Owner(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Reorder"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	vr, err := client.CreateVoiceRoom(ctx, &spacev1.CreateVoiceRoomRequest{SpaceId: spaceID, Name: "V"})
	require.NoError(t, err)

	chatA, chatB := uuid.New().String(), uuid.New().String()
	chatType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	nodeA, err := client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID, Kind: "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: chatA, Type: &chatType},
	})
	require.NoError(t, err)
	nodeB, err := client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId: spaceID, Kind: "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: chatB, Type: &chatType},
	})
	require.NoError(t, err)

	voiceNodes, _ := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	var voiceNodeID string
	for _, n := range voiceNodes.GetNodes() {
		if n.GetKind() == "voice_room" {
			voiceNodeID = n.GetId()
		}
	}
	require.NotEmpty(t, voiceNodeID)

	_, err = client.ReorderSpaceTree(ctx, &spacev1.ReorderSpaceTreeRequest{
		SpaceId:        spaceID,
		OrderedNodeIds: []string{voiceNodeID, nodeB.GetSpaceTreeNode().GetId(), nodeA.GetSpaceTreeNode().GetId()},
	})
	require.NoError(t, err)

	tree, err := client.ListSpaceTree(ctx, &spacev1.ListSpaceTreeRequest{SpaceId: spaceID})
	require.NoError(t, err)
	require.Equal(t, voiceNodeID, tree.GetNodes()[0].GetId())
	require.Equal(t, vr.GetVoiceRoom().GetId(), tree.GetNodes()[0].GetVoiceRoomId())
}

func TestCreateCategory_NonOwnerDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ownerCtx := profileFixture(t)
	_, _, otherCtx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)
	client, cleanup := startSpaceGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ownerCtx, &spacev1.CreateSpaceRequest{Name: "Owned"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	_, err = client.CreateCategory(otherCtx, &spacev1.CreateCategoryRequest{
		SpaceId: spaceID, Name: "Nope",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func ptr(s string) *string { return &s }
