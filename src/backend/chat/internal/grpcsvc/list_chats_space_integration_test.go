package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

// TestListChats_SpaceChannel_ArchivedHidden documents R3-A16: archived space chats must not
// appear in ListChats even when the viewer is a space member.
func TestListChats_SpaceChannel_ArchivedHidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	chatPool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, chatPool)
	spacePool := integrationtest.StartPostgres(t, ctx, "spacedb_list_archived", "")
	applySpaceMigration(t, ctx, spacePool)

	chatID := uuid.New()
	spaceID := uuid.New()
	owner := uuid.New()
	member := uuid.New()
	accMember := uuid.New()
	seedSpaceChannelChat(t, ctx, chatPool, spacePool, chatID, spaceID, owner, member, "general")

	_, err := chatPool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket, is_archived)
VALUES ($1, $2, 'member', 'main', true)
`, chatID, member)
	require.NoError(t, err)

	spaceMembers := &store.SpaceMembersStore{Pool: spacePool}
	client, cleanup := startChatGRPCTestServer(t, chatPool, nil, nil, nil, WithSpaceMembers(spaceMembers))
	t.Cleanup(cleanup)

	list, err := client.ListChats(withAccountProfileCtx(ctx, accMember, member), &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Empty(t, list.GetChatList().GetItems(), "archived space channel must be hidden from ListChats")
}

// TestListChats_SpaceChannel_HydratesSpaceFields documents R3-A16: space rows must hydrate Chat
// fields (space_id, threads) consistently with DM/group list rows.
func TestListChats_SpaceChannel_HydratesSpaceFields(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	chatPool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, chatPool)
	spacePool := integrationtest.StartPostgres(t, ctx, "spacedb_list_hydrate", "")
	applySpaceMigration(t, ctx, spacePool)

	chatID := uuid.New()
	spaceID := uuid.New()
	owner := uuid.New()
	member := uuid.New()
	accMember := uuid.New()
	seedSpaceChannelChat(t, ctx, chatPool, spacePool, chatID, spaceID, owner, member, "general")
	_, err := chatPool.Exec(ctx, `
UPDATE chats SET threads_enabled = true, e2e_enabled = true WHERE id = $1
`, chatID)
	require.NoError(t, err)

	spaceMembers := &store.SpaceMembersStore{Pool: spacePool}
	client, cleanup := startChatGRPCTestServer(t, chatPool, nil, nil, nil, WithSpaceMembers(spaceMembers))
	t.Cleanup(cleanup)

	list, err := client.ListChats(withAccountProfileCtx(ctx, accMember, member), &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetChatList().GetItems(), 1)
	item := list.GetChatList().GetItems()[0].GetChat()
	require.Equal(t, chatID.String(), item.GetId())
	require.Equal(t, spaceID.String(), item.GetSpaceId())
	require.True(t, item.GetThreadsEnabled())
	require.True(t, item.GetE2EEnabled())
}
