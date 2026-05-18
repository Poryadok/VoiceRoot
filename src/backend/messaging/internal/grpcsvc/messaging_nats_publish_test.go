package grpcsvc

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	messagingv1 "voice.app/voice/messaging/v1"
)

type spyMessageEvents struct {
	mu      sync.Mutex
	sent    [][3]string // message_id, chat_id, sender_profile_id
	edited  [][2]string
	deleted [][2]string
}

func (s *spyMessageEvents) PublishMessageSent(_ context.Context, messageID, chatID, senderProfileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sent = append(s.sent, [3]string{messageID, chatID, senderProfileID})
	return nil
}

func (s *spyMessageEvents) PublishMessageEdited(_ context.Context, messageID, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edited = append(s.edited, [2]string{messageID, chatID})
	return nil
}

func (s *spyMessageEvents) PublishMessageDeleted(_ context.Context, messageID, chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deleted = append(s.deleted, [2]string{messageID, chatID})
	return nil
}

func (s *spyMessageEvents) snapshot() (sent [][3]string, edited [][2]string, deleted [][2]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][3]string(nil), s.sent...), append([][2]string(nil), s.edited...), append([][2]string(nil), s.deleted...)
}

func TestMessagingGRPC_MessageEvents_SendEditDelete(t *testing.T) {
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

	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	clientID := uuid.New().String()
	sendResp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "hello nats",
		ClientMessageId: &clientID,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)
	msgID := sendResp.GetMessage().GetId()

	sent, edited, deleted := spy.snapshot()
	require.Len(t, sent, 1)
	require.Equal(t, msgID, sent[0][0])
	require.Equal(t, chatID.String(), sent[0][1])
	require.Equal(t, profA.String(), sent[0][2])
	require.Empty(t, edited)
	require.Empty(t, deleted)

	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: msgID,
		Content:   "edited",
	})
	require.NoError(t, err)

	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.NoError(t, err)

	sent, edited, deleted = spy.snapshot()
	require.Len(t, sent, 1)
	require.Len(t, edited, 1)
	require.Equal(t, msgID, edited[0][0])
	require.Equal(t, chatID.String(), edited[0][1])
	require.Len(t, deleted, 1)
	require.Equal(t, msgID, deleted[0][0])
	require.Equal(t, chatID.String(), deleted[0][1])
}
