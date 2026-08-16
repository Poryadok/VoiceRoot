package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	chatv1 "voice.app/voice/chat/v1"
)

// TestArchiveChat_HidesFromListChats documents text-chat.md §Архивирование / скрытие:
// archiving a DM hides it from the caller's main list without deleting the conversation;
// the peer still sees it; unarchive restores it for the archiver.
func TestArchiveChat_HidesFromListChats(t *testing.T) {
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

	listA, err := client.ListChats(ctxA, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, listA.GetChatList().GetItems(), 1)

	_, err = client.ArchiveChat(ctxA, &chatv1.ArchiveChatRequest{ChatId: chatID, Archived: true})
	require.NoError(t, err)

	listA, err = client.ListChats(ctxA, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Empty(t, listA.GetChatList().GetItems(), "archived DM must leave caller's ListChats")

	listB, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, listB.GetChatList().GetItems(), 1, "peer must still see the DM")

	members, err := client.ListMembers(ctxA, &chatv1.ListMembersRequest{ChatId: chatID})
	require.NoError(t, err)
	var archivedForA bool
	for _, m := range members.GetMemberList().GetMembers() {
		if m.GetProfileId() == profA.String() {
			archivedForA = m.GetIsArchived()
		}
	}
	require.True(t, archivedForA)

	_, err = client.ArchiveChat(ctxA, &chatv1.ArchiveChatRequest{ChatId: chatID, Archived: false})
	require.NoError(t, err)

	listA, err = client.ListChats(ctxA, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, listA.GetChatList().GetItems(), 1)
}

// TestMuteChat_SetsAndClearsMutedUntil documents MuteChat RPC + chat_members.muted_until.
func TestMuteChat_SetsAndClearsMutedUntil(t *testing.T) {
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
	dm, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := dm.GetChat().GetId()

	until := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	_, err = client.MuteChat(ctxA, &chatv1.MuteChatRequest{
		ChatId:     chatID,
		MutedUntil: timestamppb.New(until),
	})
	require.NoError(t, err)

	members, err := client.ListMembers(ctxA, &chatv1.ListMembersRequest{ChatId: chatID})
	require.NoError(t, err)
	var found bool
	for _, m := range members.GetMemberList().GetMembers() {
		if m.GetProfileId() != profA.String() {
			continue
		}
		found = true
		require.NotNil(t, m.GetMutedUntil())
		require.Equal(t, until, m.GetMutedUntil().AsTime().UTC().Truncate(time.Second))
	}
	require.True(t, found)

	_, err = client.MuteChat(ctxA, &chatv1.MuteChatRequest{ChatId: chatID})
	require.NoError(t, err)

	members, err = client.ListMembers(ctxA, &chatv1.ListMembersRequest{ChatId: chatID})
	require.NoError(t, err)
	for _, m := range members.GetMemberList().GetMembers() {
		if m.GetProfileId() == profA.String() {
			require.Nil(t, m.GetMutedUntil(), "omitted muted_until must unmute")
		}
	}
}

func TestArchiveChat_NonMember_NotFound(t *testing.T) {
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

	dm, err := client.CreateDM(withAccountProfileCtx(ctx, accA, profA), &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)

	_, err = client.ArchiveChat(withAccountProfileCtx(ctx, accC, profC), &chatv1.ArchiveChatRequest{
		ChatId:   dm.GetChat().GetId(),
		Archived: true,
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestMuteChat_InvalidChatID(t *testing.T) {
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

	_, err := client.MuteChat(withAccountProfileCtx(ctx, acc, prof), &chatv1.MuteChatRequest{ChatId: "not-a-uuid"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
