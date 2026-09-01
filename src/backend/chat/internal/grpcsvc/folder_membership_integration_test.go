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

// TestFolderMembershipGRPC_CustomFolderFlow documents folder membership RPCs end-to-end.
func TestFolderMembershipGRPC_CustomFolderFlow(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	peer := uuid.New()
	profiles := mapProfileAccounts{prof: acc, peer: uuid.New()}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxProf := withAccountProfileCtx(ctx, acc, prof)
	dm, err := client.CreateDM(ctxProf, &chatv1.CreateDMRequest{OtherProfileId: peer.String()})
	require.NoError(t, err)

	created, err := client.CreateFolder(ctxProf, &chatv1.CreateFolderRequest{Name: "Pinned"})
	require.NoError(t, err)
	folderID := created.GetFolder().GetId()

	_, err = client.AddChatToFolder(ctxProf, &chatv1.AddChatToFolderRequest{
		FolderId: folderID,
		ChatId:   dm.GetChat().GetId(),
	})
	require.NoError(t, err)

	list, err := client.ListChats(ctxProf, &chatv1.ListChatsRequest{FolderId: &folderID})
	require.NoError(t, err)
	require.Len(t, list.GetChatList().GetItems(), 1)

	_, err = client.PinChatInFolder(ctxProf, &chatv1.PinChatInFolderRequest{
		FolderId: folderID,
		ChatId:   dm.GetChat().GetId(),
	})
	require.NoError(t, err)

	folders, err := client.ListFolders(ctxProf, &chatv1.ListFoldersRequest{})
	require.NoError(t, err)
	require.NotEmpty(t, folders.GetFolderList().GetFolders())

	_, err = client.UnpinChatInFolder(ctxProf, &chatv1.UnpinChatInFolderRequest{
		FolderId: folderID,
		ChatId:   dm.GetChat().GetId(),
	})
	require.NoError(t, err)

	_, err = client.RemoveChatFromFolder(ctxProf, &chatv1.RemoveChatFromFolderRequest{
		FolderId: folderID,
		ChatId:   dm.GetChat().GetId(),
	})
	require.NoError(t, err)

	_, err = client.AddChatToFolder(ctxProf, &chatv1.AddChatToFolderRequest{
		FolderId: folders.GetFolderList().GetFolders()[1].GetId(),
		ChatId:   dm.GetChat().GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
