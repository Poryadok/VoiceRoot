package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

// TestGroupRoles_RemoveOwner_Forbidden documents text-chat.md simple roles: owner cannot be kicked.
func TestGroupRoles_RemoveOwner_Forbidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := profileIDs(profiles)
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Owner kick", inviteeA, inviteeB)

	_, err := client.RemoveMember(ctxFor(t, profiles, owner), &chatv1.RemoveMemberRequest{
		ChatId:    chat.GetId(),
		ProfileId: owner.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	members, err := client.ListMembers(ctxFor(t, profiles, owner), &chatv1.ListMembersRequest{
		ChatId: chat.GetId(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	roles := map[string]string{}
	for _, m := range members.GetMemberList().GetMembers() {
		roles[m.GetProfileId()] = m.GetRole()
	}
	require.Equal(t, "owner", roles[owner.String()])
}

// TestGroupRoles_LeaveChat_MemberLeaves documents voluntary leave for regular participants.
func TestGroupRoles_LeaveChat_MemberLeaves(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New(), uuid.New())
	ids := profileIDs(profiles)
	owner, inviteeA, inviteeB, leaver := ids[0], ids[1], ids[2], ids[3]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Leave", inviteeA, inviteeB)
	_, err := client.AddMembers(ctxFor(t, profiles, owner), &chatv1.AddMembersRequest{
		ChatId:     chat.GetId(),
		ProfileIds: []string{leaver.String()},
	})
	require.NoError(t, err)

	_, err = client.LeaveChat(ctxFor(t, profiles, leaver), &chatv1.LeaveChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)

	leaverCtx := ctxFor(t, profiles, leaver)
	_, err = client.GetChat(leaverCtx, &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestGroupRoles_LeaveChat_OwnerForbidden documents owner must transfer ownership before leaving.
func TestGroupRoles_LeaveChat_OwnerForbidden(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New())
	ids := profileIDs(profiles)
	owner, inviteeA, inviteeB := ids[0], ids[1], ids[2]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Owner leave", inviteeA, inviteeB)

	_, err := client.LeaveChat(ctxFor(t, profiles, owner), &chatv1.LeaveChatRequest{ChatId: chat.GetId()})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	role, err := client.ListMembers(ctxFor(t, profiles, owner), &chatv1.ListMembersRequest{
		ChatId: chat.GetId(),
		Page:   &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	for _, m := range role.GetMemberList().GetMembers() {
		if m.GetProfileId() == owner.String() {
			require.Equal(t, "owner", m.GetRole())
		}
	}
}

// TestGroupRoles_LeaveChat_PublishesLeftEvent documents chat.member_changed with change=left.
func TestGroupRoles_LeaveChat_PublishesLeftEvent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	profiles := profileMap(uuid.New(), uuid.New(), uuid.New(), uuid.New())
	ids := profileIDs(profiles)
	owner, inviteeA, inviteeB, leaver := ids[0], ids[1], ids[2], ids[3]

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	spy := &spyChatEvents{}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil, WithChatEventsPublisher(spy))
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, profiles, owner, "Leave event", inviteeA, inviteeB)
	_, err := client.AddMembers(ctxFor(t, profiles, owner), &chatv1.AddMembersRequest{
		ChatId:     chat.GetId(),
		ProfileIds: []string{leaver.String()},
	})
	require.NoError(t, err)

	_, err = client.LeaveChat(ctxFor(t, profiles, leaver), &chatv1.LeaveChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)

	_, mc := spy.snapshot()
	require.Contains(t, [][3]string(mc), [3]string{chat.GetId(), leaver.String(), "left"})
}
