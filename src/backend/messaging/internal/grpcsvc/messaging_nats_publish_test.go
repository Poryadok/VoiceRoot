package grpcsvc

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
	messagingv1 "voice.app/voice/messaging/v1"

	"voice/backend/messaging/internal/messageevents"
)

// CONTRACT_MATRIX: stream message.events (JetStream name message_events), subject message.sent; Messaging publishes; Realtime et al. subscribe.
const contractMessageSentSubject = "message.sent"

type spyMessageEvents struct {
	mu      sync.Mutex
	sent    [][3]string // message_id, chat_id, sender_profile_id
	edited  [][2]string
	deleted [][2]string
	read    [][3]string // message_id, chat_id, profile_id
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

func (s *spyMessageEvents) PublishMessageRead(_ context.Context, messageID, chatID, profileID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.read = append(s.read, [3]string{messageID, chatID, profileID})
	return nil
}

func (s *spyMessageEvents) PublishReactionAdded(_ context.Context, messageID, chatID, profileID, emoji string) error {
	return nil
}

func (s *spyMessageEvents) PublishReactionRemoved(_ context.Context, messageID, chatID, profileID, emoji string) error {
	return nil
}

func (s *spyMessageEvents) snapshot() (sent [][3]string, edited [][2]string, deleted [][2]string, read [][3]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([][3]string(nil), s.sent...), append([][2]string(nil), s.edited...), append([][2]string(nil), s.deleted...), append([][3]string(nil), s.read...)
}

func startMessagingJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { s.Shutdown() })
	return s
}

// TestMessagingGRPC_JetStream_MessageSentRoundTrip verifies SendMessage → JetStream message.sent delivery
// matches events.v1.MessageStreamEvent (CONTRACT_MATRIX message.events).
func TestMessagingGRPC_JetStream_MessageSentRoundTrip(t *testing.T) {
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

	natsSrv := startMessagingJSTestServer(t)
	natsURL := natsSrv.ClientURL()

	nc, err := nats.Connect(natsURL)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(contractMessageSentSubject)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	jsPub, err := messageevents.NewJetStreamPublisher(natsURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = jsPub.Close() })

	client, _ := startMessagingServerWired(t, pool, messagingWire{MessageEvents: jsPub})

	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	clientID := uuid.New().String()
	sendResp, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "hello jetstream",
		ClientMessageId: &clientID,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		MessageKind:     &mk,
	})
	require.NoError(t, err)
	msgID := sendResp.GetMessage().GetId()

	raw, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	var env eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(raw.Data, &env))
	require.NotEmpty(t, env.GetEventId())
	require.NotNil(t, env.GetOccurredAt())
	sent := env.GetMessageSent()
	require.NotNil(t, sent)
	require.Equal(t, msgID, sent.GetMessageId())
	require.Equal(t, chatID.String(), sent.GetChatId())
	require.Equal(t, profA.String(), sent.GetSenderProfileId())
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

	sent, edited, deleted, read := spy.snapshot()
	require.Len(t, sent, 1)
	require.Equal(t, msgID, sent[0][0])
	require.Equal(t, chatID.String(), sent[0][1])
	require.Equal(t, profA.String(), sent[0][2])
	require.Empty(t, edited)
	require.Empty(t, deleted)
	require.Empty(t, read)

	_, err = client.MarkRead(withProfileCtx(ctx, acctA, profA), &messagingv1.MarkReadRequest{
		Chat:               chatDMRef(chatID),
		LastReadMessageId: msgID,
	})
	require.NoError(t, err)

	_, err = client.EditMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.EditMessageRequest{
		MessageId: msgID,
		Content:   "edited",
	})
	require.NoError(t, err)

	_, err = client.DeleteMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.NoError(t, err)

	sent, edited, deleted, read = spy.snapshot()
	require.Len(t, sent, 1)
	require.Len(t, read, 1)
	require.Equal(t, msgID, read[0][0])
	require.Equal(t, chatID.String(), read[0][1])
	require.Equal(t, profA.String(), read[0][2])
	require.Len(t, edited, 1)
	require.Equal(t, msgID, edited[0][0])
	require.Equal(t, chatID.String(), edited[0][1])
	require.Len(t, deleted, 1)
	require.Equal(t, msgID, deleted[0][0])
	require.Equal(t, chatID.String(), deleted[0][1])
}
