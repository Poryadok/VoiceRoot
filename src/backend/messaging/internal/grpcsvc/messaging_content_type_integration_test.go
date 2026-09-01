package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	chatv1 "voice.app/voice/chat/v1"
	messagingv1 "voice.app/voice/messaging/v1"

	"voice/backend/messaging/internal/messageevents"
)

func TestMessagingSendMessage_persistsContentTypeColumn(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	ct := messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_LOCATION
	resp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		AttachmentsJson: `[{"type":"location","lat":55.75,"lon":37.61}]`,
		MentionsJson:    "[]",
		ContentType:     &ct,
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetMessage().ContentType)
	require.Equal(t, messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_LOCATION, resp.GetMessage().GetContentType())

	var stored string
	err = pool.QueryRow(ctx, `SELECT content_type FROM messages WHERE id = $1`, resp.GetMessage().GetId()).Scan(&stored)
	require.NoError(t, err)
	require.Equal(t, "location", stored)
}

func TestMessagingSendMessage_locationWithoutFileService(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	client, _ := startMessagingServer(t, pool)
	ct := messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_LOCATION
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		AttachmentsJson: `[{"type":"location","lat":10,"lon":20}]`,
		MentionsJson:    "[]",
		ContentType:     &ct,
	})
	require.NoError(t, err)
}

func TestMessagingSendMessage_messageSentIncludesContentType(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000003_attachment_only_messages.up.sql"))

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)

	natsSrv := startMessagingJSTestServer(t)
	natsURL := natsSrv.ClientURL()

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	sub, err := nc.SubscribeSync(contractMessageSentSubject)
	require.NoError(t, err)

	jsPub, err := messageevents.NewJetStreamPublisher(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = jsPub.Close() })

	client, cleanup := startMessagingServerWired(t, pool, messagingWire{MessageEvents: jsPub})
	defer cleanup()

	ct := messagingv1.MessageContentType_MESSAGE_CONTENT_TYPE_TEXT
	_, err = client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:         chatDMRef(chatID),
		Content:      "hello typed",
		MentionsJson: "[]",
		ContentType:  &ct,
	})
	require.NoError(t, err)

	msg, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	var env eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	sent := env.GetMessageSent()
	require.NotNil(t, sent)
	require.NotNil(t, sent.ContentType)
	require.Equal(t, "text", sent.GetContentType())
}

func TestMessagingPinMessage_standaloneGroupMemberDenied(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	owner := uuid.New()
	member := uuid.New()
	acctMember := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds)
VALUES ($1, 'group', $2, 0)
`, chatID, owner)
	require.NoError(t, err)
	for _, row := range []struct {
		profile uuid.UUID
		role    string
	}{
		{owner, "owner"},
		{member, "member"},
	} {
		_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, $3)`, chatID, row.profile, row.role)
		require.NoError(t, err)
	}

	client, _ := startMessagingServer(t, pool)
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	groupRef := &chatv1.ChatRef{Id: chatID.String(), Type: &group}

	msgID := uuid.New()
	_, err = pool.Exec(ctx, `
INSERT INTO messages (id, chat_id, chat_type, sender_profile_id, content, attachments, mentions)
VALUES ($1, $2, 'group', $3, 'pin me', '[]'::jsonb, '[]'::jsonb)
`, msgID, chatID, owner)
	require.NoError(t, err)

	_, err = client.PinMessage(withProfileCtx(ctx, acctMember, member), &messagingv1.PinMessageRequest{
		Chat: groupRef, MessageId: msgID.String(),
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
