package grpcsvc

import (
	"context"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"

	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// This proxy injects response loss only AFTER the real Messaging RPC commits.
// It does not implement persistence, idempotency or authorization itself.
type lostSendResponseProxy struct {
	messagingv1.UnimplementedMessagingServiceServer
	upstream messagingv1.MessagingServiceClient
	calls    atomic.Int32
}

func (s *lostSendResponseProxy) SendMessage(ctx context.Context, req *messagingv1.SendMessageRequest) (*messagingv1.SendMessageResponse, error) {
	incoming, _ := metadata.FromIncomingContext(ctx)
	response, err := s.upstream.SendMessage(metadata.NewOutgoingContext(ctx, incoming.Copy()), req)
	if s.calls.Add(1) == 1 && err == nil {
		return nil, status.Error(codes.Unavailable, "injected post-commit response loss")
	}
	return response, err
}

func TestMessagingSendMessage_RetryAfterLostResponseDoesNotDuplicate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)

	chatID, sender, recipient := uuid.New(), uuid.New(), uuid.New()
	senderAccount, recipientAccount := uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chatID, sender, recipient)
	upstream, _ := startMessagingServer(t, pool)
	proxy := &lostSendResponseProxy{upstream: upstream}
	listener := bufconn.Listen(1 << 20)
	server := grpc.NewServer()
	messagingv1.RegisterMessagingServiceServer(server, proxy)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	conn, err := grpc.NewClient("passthrough:///response-loss",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return listener.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDisableRetry(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	client := messagingv1.NewMessagingServiceClient(conn)
	clientID := uuid.NewString()
	request := &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: strings.Repeat("😀", 4000), ClientMessageId: &clientID,
	}
	senderCtx := withProfileCtx(ctx, senderAccount, sender)
	response, err := client.SendMessage(senderCtx, request)
	require.Equal(t, codes.Unavailable, status.Code(err))
	require.Nil(t, response, "the caller must not receive the committed response")
	require.EqualValues(t, 1, proxy.calls.Load(), "no automatic retry may hide the lost response")

	// Read actual committed state before explicitly retrying the identical request.
	var committedID, eventID string
	var eventHash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM messages
		WHERE chat_id=$1 AND sender_profile_id=$2 AND client_message_id=$3`, chatID, sender, clientID).Scan(&committedID))
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text,payload_sha256 FROM message_event_outbox
		WHERE chat_id=$1 AND message_id=$2 AND subject='message.sent'`, chatID, committedID).Scan(&eventID, &eventHash))

	response, err = client.SendMessage(senderCtx, request)
	require.NoError(t, err)
	require.Equal(t, committedID, response.GetMessage().GetId())
	require.Equal(t, request.Content, response.GetMessage().GetContent())
	require.EqualValues(t, 2, proxy.calls.Load())

	changed := proto.Clone(request).(*messagingv1.SendMessageRequest)
	changed.Content = "different payload at the committed client key"
	_, err = client.SendMessage(senderCtx, changed)
	require.Equal(t, codes.AlreadyExists, status.Code(err))
	require.EqualValues(t, 3, proxy.calls.Load())

	var messageCount, eventCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id=$1`, chatID).Scan(&messageCount))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE chat_id=$1`, chatID).Scan(&eventCount))
	require.Equal(t, 1, messageCount, "lost-response replay and conflicting payload must not insert another message")
	require.Equal(t, 1, eventCount, "replay and conflict must not enqueue another mutation event")
	var retainedEventID string
	var retainedEventHash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id::text,payload_sha256 FROM message_event_outbox
		WHERE chat_id=$1`, chatID).Scan(&retainedEventID, &retainedEventHash))
	require.Equal(t, eventID, retainedEventID)
	require.Equal(t, eventHash, retainedEventHash)

	history, err := upstream.GetMessages(withProfileCtx(ctx, recipientAccount, recipient), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID), Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, history.GetMessageList().GetMessages(), 1)
	require.Equal(t, committedID, history.GetMessageList().GetMessages()[0].GetId())
	require.Equal(t, request.Content, history.GetMessageList().GetMessages()[0].GetContent())
}
