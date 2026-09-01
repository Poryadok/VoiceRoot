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

// TestListFolders_SeedsSystemFolders documents chat-service.md § Folder CRUD foundation.
func TestListFolders_SeedsSystemFolders(t *testing.T) {
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

	ctxProf := withAccountProfileCtx(ctx, acc, prof)
	resp, err := client.ListFolders(ctxProf, &chatv1.ListFoldersRequest{})
	require.NoError(t, err)
	require.Len(t, resp.GetFolderList().GetFolders(), 5)
	require.Equal(t, "All", resp.GetFolderList().GetFolders()[0].GetName())
	require.Equal(t, "system", resp.GetFolderList().GetFolders()[0].GetFolderType())
}

// TestCreateFolder_CustomFolder documents custom folder creation via gRPC.
func TestCreateFolder_CustomFolder(t *testing.T) {
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

	ctxProf := withAccountProfileCtx(ctx, acc, prof)
	_, err := client.ListFolders(ctxProf, &chatv1.ListFoldersRequest{})
	require.NoError(t, err)

	created, err := client.CreateFolder(ctxProf, &chatv1.CreateFolderRequest{
		Name:             "Favorites",
		FilterConfigJson: `{"include_chat_ids":["chat-1"]}`,
	})
	require.NoError(t, err)
	require.Equal(t, "custom", created.GetFolder().GetFolderType())
	require.Equal(t, "Favorites", created.GetFolder().GetName())
	require.NotEmpty(t, created.GetFolder().GetId())

	list, err := client.ListFolders(ctxProf, &chatv1.ListFoldersRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetFolderList().GetFolders(), 6)

	_, err = client.CreateFolder(ctxProf, &chatv1.CreateFolderRequest{Name: ""})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestArchiveChat_RemovesQuickAccess documents chat-service.md § Archive side-effect on Quick Access.
func TestArchiveChat_RemovesQuickAccess(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	ctxB := withAccountProfileCtx(ctx, accB, profB)

	dm, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := dm.GetChat().GetId()

	_, err = client.AcceptDMRequest(ctxB, &chatv1.AcceptDMRequestRequest{ChatId: chatID})
	require.NoError(t, err)

	_, err = client.AddQuickAccess(ctxA, &chatv1.AddQuickAccessRequest{ChatId: chatID})
	require.NoError(t, err)

	list, err := client.ListQuickAccess(ctxA, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Len(t, list.GetItems(), 1)

	_, err = client.ArchiveChat(ctxA, &chatv1.ArchiveChatRequest{ChatId: chatID, Archived: true})
	require.NoError(t, err)

	list, err = client.ListQuickAccess(ctxA, &chatv1.ListQuickAccessRequest{})
	require.NoError(t, err)
	require.Empty(t, list.GetItems(), "archiving must remove chat from quick access")
}
