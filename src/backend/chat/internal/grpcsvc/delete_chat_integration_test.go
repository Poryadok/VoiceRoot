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

// TestDeleteChat_HidesDMForCallerOnly documents navigation.md / screen-controls §1.4 #7:
// DeleteChat is DM-only soft-delete for self; peer still sees the conversation.
func TestDeleteChat_HidesDMForCallerOnly(t *testing.T) {
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

	_, err = client.DeleteChat(ctxA, &chatv1.DeleteChatRequest{ChatId: chatID})
	require.NoError(t, err)

	listA, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Empty(t, listA.GetChatList().GetItems(), "deleted DM must leave caller's ListChats")

	listB, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, listB.GetChatList().GetItems(), 1, "peer must still see the DM")

	_, err = client.GetChat(ctxA, &chatv1.GetChatRequest{ChatId: chatID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.GetChat(ctxB, &chatv1.GetChatRequest{ChatId: chatID})
	require.NoError(t, err)

	// CreateDM restores for deleter without duplicating the chat.
	again, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.Equal(t, chatID, again.GetChat().GetId())
	listA, err = client.ListChats(ctxA, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, listA.GetChatList().GetItems(), 1)
}

// TestDeleteChat_AuthzAndTypeGuards covers unauthenticated, non-member, and non-DM rejection.
func TestDeleteChat_AuthzAndTypeGuards(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	accC := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB, profC: accC}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	ctxB := withAccountProfileCtx(ctx, accB, profB)
	ctxC := withAccountProfileCtx(ctx, accC, profC)

	_, err := client.DeleteChat(ctx, &chatv1.DeleteChatRequest{ChatId: uuid.New().String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	dm, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := dm.GetChat().GetId()
	_, err = client.AcceptDMRequest(ctxB, &chatv1.AcceptDMRequestRequest{ChatId: chatID})
	require.NoError(t, err)

	_, err = client.DeleteChat(ctxC, &chatv1.DeleteChatRequest{ChatId: chatID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	groupName := "G"
	group, err := client.CreateChat(ctxA, &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_GROUP,
		Name: &groupName,
	})
	require.NoError(t, err)
	_, err = client.AddMembers(ctxA, &chatv1.AddMembersRequest{
		ChatId:     group.GetChat().GetId(),
		ProfileIds: []string{profB.String(), profC.String()},
	})
	require.NoError(t, err)

	_, err = client.DeleteChat(ctxA, &chatv1.DeleteChatRequest{ChatId: group.GetChat().GetId()})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
