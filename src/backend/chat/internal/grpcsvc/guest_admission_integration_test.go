package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	owner, memberA, memberB, guest, newGuest := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	accounts := profileMap(owner, memberA, memberB, guest, newGuest)
	profiles := guestAdmissionProfiles{accounts: accounts, guests: map[uuid.UUID]bool{guest: true, newGuest: true}}

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

	// A previously admitted guest remains an idempotent AddMembers target after
	// disable; only a new guest admission is rejected.
	_, err = client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{
		ChatId: chat.GetId(), ProfileIds: []string{guest.String()},
	})
	require.NoError(t, err)
	_, err = client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{
		ChatId: chat.GetId(), ProfileIds: []string{newGuest.String()},
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

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

	_, err = client.RemoveMember(ctxFor(t, accounts, owner), &chatv1.RemoveMemberRequest{ChatId: chat.GetId(), ProfileId: guest.String()})
	require.NoError(t, err)
	_, err = client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{ChatId: chat.GetId(), ProfileIds: []string{guest.String()}})
	require.NoError(t, err)
	_, err = client.LeaveChat(withGuestAccountProfileCtx(context.Background(), accounts[guest], guest), &chatv1.LeaveChatRequest{ChatId: chat.GetId()})
	require.NoError(t, err)
}

func TestGuestAdmission_DisableWinsAgainstBlockedAdmission(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	owner, memberA, memberB, guest := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	accounts := profileMap(owner, memberA, memberB, guest)
	profiles := guestAdmissionProfiles{accounts: accounts, guests: map[uuid.UUID]bool{guest: true}}

	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)
	chat := createStandaloneGroup(t, client, accounts, owner, "Admission lock", memberA, memberB)
	allow := true
	_, err := client.UpdateChat(ctxFor(t, accounts, owner), &chatv1.UpdateChatRequest{ChatId: chat.GetId(), AllowGuests: &allow})
	require.NoError(t, err)

	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = tx.Exec(ctx, `SELECT id FROM chats WHERE id = $1 FOR UPDATE`, chat.GetId())
	require.NoError(t, err)

	addDone := make(chan error, 1)
	go func() {
		_, addErr := client.AddMembers(ctxFor(t, accounts, owner), &chatv1.AddMembersRequest{ChatId: chat.GetId(), ProfileIds: []string{guest.String()}})
		addDone <- addErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `
SELECT EXISTS (
  SELECT 1 FROM pg_stat_activity
  WHERE wait_event_type = 'Lock' AND query LIKE '%allow_guests%'
)`).Scan(&waiting)
		return err == nil && waiting
	}, time.Second, 10*time.Millisecond)
	_, err = tx.Exec(ctx, `UPDATE chats SET allow_guests = false WHERE id = $1`, chat.GetId())
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.Equal(t, codes.PermissionDenied, status.Code(<-addDone))
}

func TestGuestAdmissionMigration_BackfillsOnlyStandaloneChats(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	for _, name := range []string{
		"000001_init.up.sql", "000002_dm_requests.up.sql", "000003_groups.up.sql",
		"000004_slow_mode.up.sql", "000005_thread_settings.up.sql", "000006_e2e_enabled.up.sql",
		"000007_allow_guests.up.sql", "000008_folders.up.sql", "000009_folder_chats.up.sql",
		"000010_quick_access_chats.up.sql", "000011_deleted_for_self.up.sql",
	} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}

	creator, member := uuid.New(), uuid.New()
	standaloneGroup, standaloneChannel, spaceChannel := uuid.New(), uuid.New(), uuid.New()
	spaceID := uuid.New()
	for _, row := range []struct {
		id       uuid.UUID
		chatType string
		spaceID  *uuid.UUID
	}{
		{standaloneGroup, "group", nil},
		{standaloneChannel, "channel", nil},
		{spaceChannel, "channel", &spaceID},
	} {
		_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, space_id, name, creator_profile_id, allow_guests)
VALUES ($1, $2, $3, 'legacy guest chat', $4, true)
`, row.id, row.chatType, row.spaceID, creator)
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role, inbox_bucket) VALUES ($1, $2, 'owner', 'main'), ($1, $3, 'member', 'main')`, standaloneGroup, creator, member)
	require.NoError(t, err)

	body, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", "000012_allow_guests_fail_closed.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(body))
	require.NoError(t, err)

	for _, chatID := range []uuid.UUID{standaloneGroup, standaloneChannel} {
		var allowed bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT allow_guests FROM chats WHERE id = $1`, chatID).Scan(&allowed))
		require.False(t, allowed)
	}
	var spaceAllowed bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT allow_guests FROM chats WHERE id = $1`, spaceChannel).Scan(&spaceAllowed))
	require.True(t, spaceAllowed)
	var memberCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM chat_members WHERE chat_id = $1`, standaloneGroup).Scan(&memberCount))
	require.Equal(t, 2, memberCount)
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
