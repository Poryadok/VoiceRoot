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

// guestAdmissionProfiles is a User-owned fixture: only this lookup determines
// whether an invited profile is a guest. The caller account type comes from
// signed Gateway metadata, as it does in production.
type guestAdmissionProfiles struct {
	accounts mapProfileAccounts
	guests   map[uuid.UUID]bool
}

func (p guestAdmissionProfiles) AccountIDByProfileID(ctx context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	return p.accounts.AccountIDByProfileID(ctx, profileID)
}

func (p guestAdmissionProfiles) IsGuestProfile(_ context.Context, profileID uuid.UUID) (bool, error) {
	if _, ok := p.accounts[profileID]; !ok {
		return false, status.Error(codes.NotFound, "profile not found")
	}
	return p.guests[profileID], nil
}

func TestGuestAdmission_StandaloneGroupIsForwardOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, memberA, memberB, guest := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	accounts := profileMap(owner, memberA, memberB, guest)
	profiles := guestAdmissionProfiles{accounts: accounts, guests: map[uuid.UUID]bool{guest: true}}

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	chat := createStandaloneGroup(t, client, accounts, owner, "Guests", memberA, memberB)
	require.False(t, chat.GetAllowGuests())

	_, err := client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{
		ChatId: chat.GetId(), ProfileIds: []string{guest.String()},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	allow := true
	updated, err := client.UpdateChat(ctxFor(t, accounts, owner), &chatv1.UpdateChatRequest{
		ChatId: chat.GetId(), AllowGuests: &allow,
	})
	require.NoError(t, err)
	require.True(t, updated.GetChat().GetAllowGuests())

	_, err = client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{
		ChatId: chat.GetId(), ProfileIds: []string{guest.String()},
	})
	require.NoError(t, err)

	allow = false
	_, err = client.UpdateChat(ctxFor(t, accounts, owner), &chatv1.UpdateChatRequest{
		ChatId: chat.GetId(), AllowGuests: &allow,
	})
	require.NoError(t, err)

	// Disabling is forward-only: a previously admitted guest remains a member.
	_, err = client.GetChat(withGuestAccountProfileCtx(context.Background(), accounts[guest], guest), &chatv1.GetChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
}

func TestGuestAdmission_StandaloneChannelAndGuestInitiatorDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, guest := uuid.New(), uuid.New()
	accounts := profileMap(owner, guest)
	profiles := guestAdmissionProfiles{accounts: accounts, guests: map[uuid.UUID]bool{guest: true}}

	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	for _, typ := range []chatv1.ChatType{chatv1.ChatType_CHAT_TYPE_GROUP, chatv1.ChatType_CHAT_TYPE_CHANNEL} {
		name := "Guest cannot create"
		_, err := client.CreateChat(withGuestAccountProfileCtx(context.Background(), accounts[guest], guest), &chatv1.CreateChatRequest{Type: typ, Name: &name})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "type %s", typ)
	}

	name := "Announcements"
	created, err := client.CreateChat(ctxFor(t, accounts, owner), &chatv1.CreateChatRequest{Type: chatv1.ChatType_CHAT_TYPE_CHANNEL, Name: &name})
	require.NoError(t, err)
	chat := created.GetChat()
	require.False(t, chat.GetAllowGuests())

	_, err = client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{ChatId: chat.GetId(), ProfileIds: []string{guest.String()}})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	allow := true
	_, err = client.UpdateChat(ctxFor(t, accounts, owner), &chatv1.UpdateChatRequest{ChatId: chat.GetId(), AllowGuests: &allow})
	require.NoError(t, err)
	_, err = client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{ChatId: chat.GetId(), ProfileIds: []string{guest.String()}})
	require.NoError(t, err)
}

func TestGuestAdmission_SpaceChatsRemainOwnedBySpaceAndRole(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner := uuid.New()
	accounts := profileMap(owner)
	pool := startChatPostgresForTest(t, context.Background())
	applyChatMigration(t, context.Background(), pool)
	client, cleanup := startChatGRPCTestServer(t, pool, accounts, nil, nil)
	t.Cleanup(cleanup)

	name, spaceID := "Space channel", uuid.New().String()
	created, err := client.CreateChat(ctxFor(t, accounts, owner), &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_CHANNEL, Name: &name, SpaceId: &spaceID,
	})
	require.NoError(t, err)
	allow := true
	_, err = client.UpdateChat(ctxFor(t, accounts, owner), &chatv1.UpdateChatRequest{ChatId: created.GetChat().GetId(), AllowGuests: &allow})
	// The chat layer has no membership authority for Space chats, so this is
	// denied before a standalone setting can be written.
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
