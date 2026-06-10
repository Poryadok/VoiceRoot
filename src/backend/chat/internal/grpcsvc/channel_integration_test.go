package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
)

func TestCreateChannel_SpaceChannel_NoChatMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New())
	owner := uuidKey(profiles)
	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	name := "Announcements"
	spaceID := uuid.New().String()
	resp, err := client.CreateChat(ctxFor(t, profiles, owner), &chatv1.CreateChatRequest{
		Type:    chatv1.ChatType_CHAT_TYPE_CHANNEL,
		Name:    &name,
		SpaceId: &spaceID,
	})
	require.NoError(t, err)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, resp.GetChat().GetType())
	require.Equal(t, spaceID, resp.GetChat().GetSpaceId())
	require.Equal(t, name, resp.GetChat().GetName())

	var memberCount int
	err = pool.QueryRow(context.Background(), `
SELECT COUNT(*) FROM chat_members WHERE chat_id = $1
`, resp.GetChat().GetId()).Scan(&memberCount)
	require.NoError(t, err)
	require.Zero(t, memberCount, "space channels inherit members from space_members, not chat_members")
}

func TestCreateChannel_Standalone_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New())
	owner := uuidKey(profiles)
	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	name := "Orphan channel"
	_, err := client.CreateChat(ctxFor(t, profiles, owner), &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_CHANNEL,
		Name: &name,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
