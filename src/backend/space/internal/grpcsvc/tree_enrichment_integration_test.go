package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/space/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	spacev1 "voice.app/voice/space/v1"
)

func TestListSpaceTree_enrichesTextChatFromChatLookup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	_, _, ctx := profileFixture(t)
	pool := startSpacePostgresForTest(t, context.Background())
	applySpaceMigration(t, context.Background(), pool)

	groupChatID := uuid.New()
	channelChatID := uuid.New()
	lookup := &mapChatLookup{chats: map[uuid.UUID]ChatInfo{
		groupChatID: {
			Name:     "Raid planning",
			ChatType: chatv1.ChatType_CHAT_TYPE_GROUP,
		},
		channelChatID: {
			Name:     "Announcements",
			ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL,
		},
	}}
	client, cleanup := startSpaceGRPCTestServer(t, pool, withSpaceChatLookup(lookup))
	t.Cleanup(cleanup)

	created, err := client.CreateSpace(ctx, &spacev1.CreateSpaceRequest{Name: "Enriched"})
	require.NoError(t, err)
	spaceID := created.GetSpace().GetId()

	groupType := chatv1.ChatType_CHAT_TYPE_GROUP
	_, err = client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId:    spaceID,
		Kind:       "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: groupChatID.String(), Type: &groupType},
	})
	require.NoError(t, err)

	channelType := chatv1.ChatType_CHAT_TYPE_CHANNEL
	_, err = client.UpsertTreeNode(ctx, &spacev1.UpsertTreeNodeRequest{
		SpaceId:    spaceID,
		Kind:       "text_chat",
		LinkedChat: &chatv1.ChatRef{Id: channelChatID.String(), Type: &channelType},
	})
	require.NoError(t, err)

	spaceUUID, err := uuid.Parse(spaceID)
	require.NoError(t, err)
	data, err := (&store.SpaceStore{Pool: pool}).ListSpaceTree(ctx, spaceUUID)
	require.NoError(t, err)

	svc := &SpaceGRPC{Store: &store.SpaceStore{Pool: pool}, Chats: lookup}
	tree := svc.spaceTreeDataToProto(ctx, data)

	var groupNode, channelNode *spacev1.SpaceTreeNode
	for _, n := range tree.GetNodes() {
		if n.GetKind() != "text_chat" {
			continue
		}
		switch n.GetLinkedChat().GetId() {
		case groupChatID.String():
			groupNode = n
		case channelChatID.String():
			channelNode = n
		}
	}
	require.NotNil(t, groupNode)
	require.Equal(t, "Raid planning", groupNode.GetDisplayName())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, groupNode.GetLinkedChat().GetType())

	require.NotNil(t, channelNode)
	require.Equal(t, "Announcements", channelNode.GetDisplayName())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, channelNode.GetLinkedChat().GetType())
}
