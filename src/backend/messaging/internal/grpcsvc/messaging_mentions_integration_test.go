package grpcsvc

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	messagingv1 "voice.app/voice/messaging/v1"

	"voice/backend/role/permissions"
)

type testRolePerms struct {
	allowed bool
}

func (t testRolePerms) HasSpacePermission(context.Context, uuid.UUID, uuid.UUID, string) (bool, error) {
	return t.allowed, nil
}

func (t testRolePerms) HasChatPermission(context.Context, uuid.UUID, uuid.UUID, uuid.UUID, string) (bool, error) {
	return t.allowed, nil
}

type testPresence struct {
	online []uuid.UUID
}

func (t testPresence) FilterOnlineProfileIDs(context.Context, []uuid.UUID) ([]uuid.UUID, error) {
	return append([]uuid.UUID(nil), t.online...), nil
}

// TestMessagingSendMessage_dmUserMention documents PLAN Phase 6 / text-chat.md @user in DM.
func TestMessagingSendMessage_dmUserMention(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	spy := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{MessageEvents: spy})

	mentionsJSON := `[{"type":"user","target_id":"` + profB.String() + `"}]`
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "hey @" + profB.String(),
		AttachmentsJson: "[]",
		MentionsJson:    mentionsJSON,
	})
	require.NoError(t, err)
	require.JSONEq(t, mentionsJSON, sent.GetMessage().GetMentionsJson())

	snap, mentionEv, _, _, _ := spy.snapshot()
	require.Len(t, snap, 1)
	require.Equal(t, "true", snap[0][3])
	require.Len(t, mentionEv, 1)
	require.Equal(t, profB.String(), mentionEv[0][3])
}

// TestMessagingSendMessage_mentionNonMemberRejected ensures unknown profile cannot be mentioned.
func TestMessagingSendMessage_mentionNonMemberRejected(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	stranger := uuid.New()
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "hey",
		AttachmentsJson: "[]",
		MentionsJson:    `[{"type":"user","target_id":"` + stranger.String() + `"}]`,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestMessagingSendMessage_everyoneInSpaceRequiresPermission documents @everyone role gate.
func TestMessagingSendMessage_everyoneInSpaceRequiresPermission(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)

	client, _ := startMessagingServerWired(t, pool, messagingWire{RolePermissions: testRolePerms{allowed: false}})
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "@everyone hi",
		AttachmentsJson: "[]",
		MentionsJson:    `[{"type":"everyone"}]`,
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestMessagingSendMessage_everyoneInSpaceAllowed stores mention and publishes targets.
func TestMessagingSendMessage_everyoneInSpaceAllowed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	applyModerationSchemasForMessagingTest(t, ctx, pool)
	_, err := pool.Exec(ctx, `UPDATE chats SET space_id = $2 WHERE id = $1`, chatID, spaceID)
	require.NoError(t, err)

	spy := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		MessageEvents:   spy,
		RolePermissions: testRolePerms{allowed: true},
	})
	sent, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "@everyone hi",
		AttachmentsJson: "[]",
		MentionsJson:    `[{"type":"everyone"}]`,
	})
	require.NoError(t, err)
	var stored []map[string]string
	require.NoError(t, json.Unmarshal([]byte(sent.GetMessage().GetMentionsJson()), &stored))
	require.Len(t, stored, 1)
	require.Equal(t, "everyone", stored[0]["type"])

	_, mentionEv, _, _, _ := spy.snapshot()
	require.Len(t, mentionEv, 1)
	require.Contains(t, mentionEv[0][3], profB.String())
	_ = permissions.TextChatMentionAllInChat
}

// TestMessagingSendMessage_hereUsesPresence documents @here online filter.
func TestMessagingSendMessage_hereUsesPresence(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))

	chatID := uuid.New()
	spaceID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profC := uuid.New()
	acctA := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds, space_id)
VALUES ($1, 'group', $2, 0, $3)
`, chatID, profA, spaceID)
	require.NoError(t, err)
	for _, p := range []uuid.UUID{profA, profB, profC} {
		_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, p)
		require.NoError(t, err)
	}
	applyModerationSchemasForMessagingTest(t, ctx, pool)

	spy := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{
		MessageEvents:   spy,
		RolePermissions: testRolePerms{allowed: true},
		UserPresence:    testPresence{online: []uuid.UUID{profC}},
	})
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "@here ping",
		AttachmentsJson: "[]",
		MentionsJson:    `[{"type":"here"}]`,
	})
	require.NoError(t, err)

	_, mentionEv, _, _, _ := spy.snapshot()
	require.Len(t, mentionEv, 1)
	require.Equal(t, profC.String(), mentionEv[0][3])
}
