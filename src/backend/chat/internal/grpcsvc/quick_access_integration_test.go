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

// TestQuickAccess_ListAddRemoveReorder documents chat-service.md § Quick Access RPCs.
func TestQuickAccess_ListAddRemoveReorder(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	peers := make([]uuid.UUID, 16)
	for i := range peers {
		peers[i] = uuid.New()
	}
	profiles := mapProfileAccounts{prof: acc}
	for _, peer := range peers {
		profiles[peer] = uuid.New()
	}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxProf := withAccountProfileCtx(ctx, acc, prof)

	chatIDs := make([]string, 0, 16)
	for _, peer := range peers {
		dm, err := client.CreateDM(ctxProf, &chatv1.CreateDMRequest{OtherProfileId: peer.String()})
		require.NoError(t, err)
		chatIDs = append(chatIDs, dm.GetChat().GetId())
	}

	empty, err := client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Empty(t, empty.GetItems())

	for i := 0; i < 15; i++ {
		_, err = client.AddQuickAccess(ctxProf, &chatv1.AddQuickAccessRequest{ChatId: chatIDs[i]})
		require.NoError(t, err)
	}

	_, err = client.AddQuickAccess(ctxProf, &chatv1.AddQuickAccessRequest{ChatId: chatIDs[15]})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	list, err := client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetItems(), 15)
	require.NotNil(t, list.GetItems()[0].GetChat())

	_, err = client.AddQuickAccess(ctxProf, &chatv1.AddQuickAccessRequest{ChatId: chatIDs[0]})
	require.NoError(t, err)

	_, err = client.ReorderQuickAccess(ctxProf, &chatv1.ReorderQuickAccessRequest{
		ChatIds: append([]string{chatIDs[2], chatIDs[0], chatIDs[1]}, chatIDs[3:15]...),
	})
	require.NoError(t, err)

	list, err = client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Equal(t, chatIDs[2], list.GetItems()[0].GetChatId())

	_, err = client.RemoveQuickAccess(ctxProf, &chatv1.RemoveQuickAccessRequest{ChatId: chatIDs[2]})
	require.NoError(t, err)

	list, err = client.ListQuickAccess(ctxProf, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetItems(), 14)
}

func TestQuickAccess_AddNonMember_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	client, cleanup := startChatGRPCTestServer(t, pool, mapProfileAccounts{prof: acc}, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.AddQuickAccess(withAccountProfileCtx(ctx, acc, prof), &chatv1.AddQuickAccessRequest{
		ChatId: uuid.New().String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
