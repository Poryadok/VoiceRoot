package grpcsvc

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/messaging/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

type faultGuard struct {
	memberErr error
	peerErr   error
	peer      uuid.UUID
}

func (g faultGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return g.memberErr
}

func (g faultGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	if g.peerErr != nil {
		return uuid.Nil, g.peerErr
	}
	return g.peer, nil
}

func closedPoolSvc(t *testing.T) (*MessagingGRPC, context.Context, uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	pool.Close()
	acct := uuid.New()
	prof := uuid.New()
	svc := &MessagingGRPC{Messages: &store.MessagesStore{Pool: pool}}
	return svc, profileCtx(acct, prof), acct, prof
}

func TestMessagingGRPC_handlerPreconditionsAndFaults(t *testing.T) {
	t.Parallel()

	t.Run("EditMessage missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.EditMessage(context.Background(), &messagingv1.EditMessageRequest{MessageId: uuid.New().String(), Content: "x"})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("DeleteMessage missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.DeleteMessage(context.Background(), &messagingv1.DeleteMessageRequest{MessageId: uuid.New().String()})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("GetMessages missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.GetMessages(context.Background(), &messagingv1.GetMessagesRequest{Chat: chatDMRef(uuid.New())})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("MarkRead missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.MarkRead(context.Background(), &messagingv1.MarkReadRequest{Chat: chatDMRef(uuid.New()), LastReadMessageId: uuid.New().String()})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("GetReadState missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.GetReadState(context.Background(), &messagingv1.GetReadStateRequest{Chat: chatDMRef(uuid.New())})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("GetBulkReadState missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.GetBulkReadState(context.Background(), &messagingv1.GetBulkReadStateRequest{})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})

	t.Run("GetChatListMetadata missing profile", func(t *testing.T) {
		t.Parallel()
		svc := &MessagingGRPC{Messages: &store.MessagesStore{}}
		_, err := svc.GetChatListMetadata(context.Background(), &messagingv1.GetChatListMetadataRequest{})
		require.Equal(t, codes.Unauthenticated, status.Code(err))
	})
}

func TestMessagingGRPC_closedPoolErrors(t *testing.T) {
	svc, ctx, _, _ := closedPoolSvc(t)
	chatID := uuid.New()
	chat := chatDMRef(chatID)
	msgID := uuid.New().String()

	_, err := svc.EditMessage(ctx, &messagingv1.EditMessageRequest{MessageId: msgID, Content: "x"})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.DeleteMessage(ctx, &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.GetMessages(ctx, &messagingv1.GetMessagesRequest{Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 10}})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.MarkRead(ctx, &messagingv1.MarkReadRequest{Chat: chat, LastReadMessageId: msgID})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.GetReadState(ctx, &messagingv1.GetReadStateRequest{Chat: chat})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.GetBulkReadState(ctx, &messagingv1.GetBulkReadStateRequest{Chats: []*chatv1.ChatRef{chat}})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.GetChatListMetadata(ctx, &messagingv1.GetChatListMetadataRequest{Chats: []*chatv1.ChatRef{chat}})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestMessagingGRPC_chatGuardInternalErrors(t *testing.T) {
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

	internal := errors.New("chat svc internal")
	svc := &MessagingGRPC{
		Messages:  &store.MessagesStore{Pool: pool},
		ChatGuard: faultGuard{memberErr: internal},
	}
	pctx := profileCtx(acctA, profA)

	_, err := svc.GetMessages(pctx, &messagingv1.GetMessagesRequest{Chat: chatDMRef(chatID), Page: &commonv1.CursorPageRequest{PageSize: 10}})
	require.Equal(t, codes.Internal, status.Code(err))

	_, err = svc.GetBulkReadState(pctx, &messagingv1.GetBulkReadStateRequest{Chats: []*chatv1.ChatRef{chatDMRef(chatID)}})
	require.Equal(t, codes.Internal, status.Code(err))
}

func TestMessagingGRPC_editDeleteValidation(t *testing.T) {
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
	mk := messagingv1.MessageKind_MESSAGE_KIND_REGULAR
	pctx := withProfileCtx(ctx, acctA, profA)
	send, err := client.SendMessage(pctx, &messagingv1.SendMessageRequest{
		Chat: chatDMRef(chatID), Content: "x", AttachmentsJson: "[]", MentionsJson: "[]", MessageKind: &mk,
	})
	require.NoError(t, err)
	msgID := send.GetMessage().GetId()

	_, err = client.EditMessage(pctx, &messagingv1.EditMessageRequest{MessageId: msgID, Content: stringsRepeat("y", 4001)})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = client.EditMessage(pctx, &messagingv1.EditMessageRequest{MessageId: uuid.New().String(), Content: "nope"})
	require.Equal(t, codes.NotFound, status.Code(err))

	edited, err := client.EditMessage(pctx, &messagingv1.EditMessageRequest{MessageId: msgID, Content: "edited"})
	require.NoError(t, err)
	require.Equal(t, "edited", edited.GetMessage().GetContent())

	_, err = client.DeleteMessage(pctx, &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.NoError(t, err)

	_, err = client.DeleteMessage(pctx, &messagingv1.DeleteMessageRequest{MessageId: msgID})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func stringsRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
