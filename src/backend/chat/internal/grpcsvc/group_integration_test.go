package grpcsvc

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

const (
	groupMemberLimit = 500
	minGroupMembers  = 3
)

func profileMap(ids ...uuid.UUID) mapProfileAccounts {
	m := make(mapProfileAccounts, len(ids))
	for _, id := range ids {
		m[id] = uuid.New()
	}
	return m
}

func ctxFor(t *testing.T, profiles mapProfileAccounts, profileID uuid.UUID) context.Context {
	t.Helper()
	acc, ok := profiles[profileID]
	require.True(t, ok, "profile %s missing from test fixture", profileID)
	return withAccountProfileCtx(context.Background(), acc, profileID)
}

func createStandaloneGroup(
	t *testing.T,
	client chatv1.ChatServiceClient,
	profiles mapProfileAccounts,
	owner uuid.UUID,
	name string,
	initialInvitees ...uuid.UUID,
) *chatv1.Chat {
	t.Helper()
	require.GreaterOrEqual(t, 1+len(initialInvitees), minGroupMembers,
		"standalone group needs creator plus enough invitees for %d members", minGroupMembers)

	ctx := ctxFor(t, profiles, owner)
	groupName := name
	created, err := client.CreateChat(ctx, &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_GROUP,
		Name: &groupName,
	})
	require.NoError(t, err)
	chat := created.GetChat()
	require.NotNil(t, chat)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, chat.GetType())
	require.Equal(t, groupName, chat.GetName())
	require.Equal(t, owner.String(), chat.GetCreatorProfileId())
	require.Nil(t, chat.SpaceId)

	if len(initialInvitees) == 0 {
		return chat
	}
	ids := make([]string, 0, len(initialInvitees))
	for _, id := range initialInvitees {
		ids = append(ids, id.String())
	}
	_, err = client.AddMembers(ctx, &chatv1.AddMembersRequest{
		ChatId:     chat.GetId(),
		ProfileIds: ids,
	})
	require.NoError(t, err)
	return chat
}

func seedGroupMembers(
	t *testing.T,
	ctx context.Context,
	pool *pgxpool.Pool,
	chatID uuid.UUID,
	owner uuid.UUID,
	extra int,
) {
	t.Helper()
	for i := 0; i < extra; i++ {
		profileID := uuid.New()
		_, err := pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role)
VALUES ($1, $2, 'member')
`, chatID, profileID)
		require.NoError(t, err)
	}
}

// TestCreateGroup_StandaloneMinimumThreeMembers documents PLAN Phase 4 / text-chat.md:
// standalone group (no space_id), creator is owner, at least three members total.
func TestCreateGroup_StandaloneMinimumThreeMembers(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Friday squad", inviteeA, inviteeB)

	members, err := client.ListMembers(ctxFor(t, profiles, owner), &chatv1.ListMembersRequest{
		ChatId: chat.GetId(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, members.GetMemberList().GetMembers(), minGroupMembers)

	roles := map[string]string{}
	for _, m := range members.GetMemberList().GetMembers() {
		roles[m.GetProfileId()] = m.GetRole()
	}
	require.Equal(t, "owner", roles[owner.String()])
	require.Equal(t, "member", roles[inviteeA.String()])
	require.Equal(t, "member", roles[inviteeB.String()])
}

// TestCreateGroup_TwoMembersOnly_InvalidArgument documents text-chat.md: two participants is a DM, not a group.
func TestCreateGroup_TwoMembersOnly_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 2)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, onlyInvitee := ids[0], ids[1]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctx := ctxFor(t, profiles, owner)
	groupName := "Pair"
	created, err := client.CreateChat(ctx, &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_GROUP,
		Name: &groupName,
	})
	require.NoError(t, err)

	_, err = client.AddMembers(ctx, &chatv1.AddMembersRequest{
		ChatId:     created.GetChat().GetId(),
		ProfileIds: []string{onlyInvitee.String()},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateGroup_AddMembers_Invite documents PLAN Phase 4 invite: members can join via AddMembers.
func TestCreateGroup_AddMembers_Invite(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 4)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB, lateJoiner := ids[0], ids[1], ids[2], ids[3]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Raid", inviteeA, inviteeB)

	_, err := client.AddMembers(ctxFor(t, profiles, inviteeA), &chatv1.AddMembersRequest{
		ChatId:     chat.GetId(),
		ProfileIds: []string{lateJoiner.String()},
	})
	require.NoError(t, err)

	lateCtx := ctxFor(t, profiles, lateJoiner)
	got, err := client.GetChat(lateCtx, &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, got.GetChat().GetType())

	list, err := client.ListChats(lateCtx, &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	found := false
	for _, item := range list.GetChatList().GetItems() {
		if item.GetChat().GetId() == chat.GetId() {
			found = true
			break
		}
	}
	require.True(t, found, "invited member must see group in ListChats")
}

// TestCreateGroup_RemoveMember_Kick documents PLAN Phase 4 kick via RemoveMember.
func TestCreateGroup_RemoveMember_Kick(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Kick test", inviteeA, inviteeB)

	_, err := client.RemoveMember(ctxFor(t, profiles, owner), &chatv1.RemoveMemberRequest{
		ChatId:    chat.GetId(),
		ProfileId: inviteeB.String(),
	})
	require.NoError(t, err)

	kickedCtx := ctxFor(t, profiles, inviteeB)
	_, err = client.GetChat(kickedCtx, &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.ListMembers(kickedCtx, &chatv1.ListMembersRequest{
		ChatId: chat.GetId(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateGroup_RemoveMember_NonOwnerForbidden documents kick is not available to regular members.
func TestCreateGroup_RemoveMember_NonOwnerForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Permissions", inviteeA, inviteeB)

	_, err := client.RemoveMember(ctxFor(t, profiles, inviteeA), &chatv1.RemoveMemberRequest{
		ChatId:    chat.GetId(),
		ProfileId: inviteeB.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestCreateGroup_UpdateChat_Avatar documents PLAN Phase 4 group avatar via UpdateChat.avatar_url.
func TestCreateGroup_UpdateChat_Avatar(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Avatar group", inviteeA, inviteeB)
	avatar := "https://cdn.voice.gg/groups/party.webp"

	updated, err := client.UpdateChat(ctxFor(t, profiles, owner), &chatv1.UpdateChatRequest{
		ChatId:    chat.GetId(),
		AvatarUrl: &avatar,
	})
	require.NoError(t, err)
	require.Equal(t, avatar, updated.GetChat().GetAvatarUrl())

	got, err := client.GetChat(ctxFor(t, profiles, inviteeA), &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
	require.Equal(t, avatar, got.GetChat().GetAvatarUrl())
}

// TestCreateGroup_MemberLimit500 documents chat-service.md / text-chat.md: standalone groups cap at 500 members.
func TestCreateGroup_MemberLimit500(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]
	extraA, extraB := uuid.New(), uuid.New()
	profiles[extraA] = uuid.New()
	profiles[extraB] = uuid.New()

	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Full house", inviteeA, inviteeB)
	chatID, err := uuid.Parse(chat.GetId())
	require.NoError(t, err)

	// Fill to capacity minus one so the next invite would exceed the product limit.
	seedGroupMembers(t, ctx, pool, chatID, owner, groupMemberLimit-minGroupMembers-1)

	_, err = client.AddMembers(ctxFor(t, profiles, owner), &chatv1.AddMembersRequest{
		ChatId:     chat.GetId(),
		ProfileIds: []string{extraA.String(), extraB.String()},
	})
	require.Error(t, err)
	code := status.Code(err)
	require.True(t, code == codes.FailedPrecondition || code == codes.ResourceExhausted,
		"member limit must reject overflow, got %v (%s)", code, err)
	require.True(t, strings.Contains(strings.ToLower(status.Convert(err).Message()), "500") ||
		strings.Contains(strings.ToLower(status.Convert(err).Message()), "limit") ||
		strings.Contains(strings.ToLower(status.Convert(err).Message()), "full"),
		"overflow error should mention capacity, got %q", status.Convert(err).Message())
}

// TestCreateGroup_ChatEvents documents chat-service.md NATS events for group lifecycle.
func TestCreateGroup_ChatEvents(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 4)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB, kicked := ids[0], ids[1], ids[2], ids[3]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	spy := &spyChatEvents{}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithChatEventsPublisher(spy))
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Events", inviteeA, inviteeB)
	chatID := chat.GetId()

	cr, mc := spy.snapshot()
	require.Len(t, cr, 1)
	require.Equal(t, chatID, cr[0][0])
	require.Equal(t, "group", cr[0][1])
	require.GreaterOrEqual(t, len(mc), minGroupMembers)

	_, err := client.AddMembers(ctxFor(t, profiles, owner), &chatv1.AddMembersRequest{
		ChatId:     chatID,
		ProfileIds: []string{kicked.String()},
	})
	require.NoError(t, err)
	_, mc = spy.snapshot()
	require.Contains(t, [][3]string(mc), [3]string{chatID, kicked.String(), "joined"})

	_, err = client.RemoveMember(ctxFor(t, profiles, owner), &chatv1.RemoveMemberRequest{
		ChatId:    chatID,
		ProfileId: kicked.String(),
	})
	require.NoError(t, err)
	_, mc = spy.snapshot()
	require.Contains(t, [][3]string(mc), [3]string{chatID, kicked.String(), "removed"})
}

// TestCreateGroup_ListChats_IncludesGroup ensures group chats surface in the main inbox list.
func TestCreateGroup_ListChats_IncludesGroup(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := make([]uuid.UUID, 0, 3)
	for id := range profiles {
		ids = append(ids, id)
	}
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "List me", inviteeA, inviteeB)

	list, err := client.ListChats(ctxFor(t, profiles, owner), &chatv1.ListChatsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.NotEmpty(t, list.GetChatList().GetItems())
	found := false
	for _, item := range list.GetChatList().GetItems() {
		if item.GetChat().GetId() != chat.GetId() {
			continue
		}
		found = true
		require.Equal(t, chatv1.ChatType_CHAT_TYPE_GROUP, item.GetChat().GetType())
		require.Equal(t, "List me", item.GetChat().GetName())
	}
	require.True(t, found, fmt.Sprintf("group %s missing from ListChats", chat.GetId()))
}
