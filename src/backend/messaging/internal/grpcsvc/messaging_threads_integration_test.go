package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// applyThreadMessagingMigrations applies chat + messaging schemas used by Phase 10 thread tests.
func applyThreadMessagingMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000005_thread_settings.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000007_thread_index.up.sql"))
}

func setChatThreadSettings(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID uuid.UUID, threadsEnabled, allowUserMainFeed bool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
UPDATE chats
SET threads_enabled = $2, allow_user_main_feed = $3
WHERE id = $1
`, chatID, threadsEnabled, allowUserMainFeed)
	require.NoError(t, err)
}

func seedChannelChat(t *testing.T, ctx context.Context, pool *pgxpool.Pool, chatID, owner uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO chats (id, type, creator_profile_id, slow_mode_seconds, threads_enabled, allow_user_main_feed)
VALUES ($1, 'channel', $2, 0, true, false)
`, chatID, owner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')
`, chatID, owner)
	require.NoError(t, err)
}

func sendPostedAsChatDirect(t *testing.T, ctx context.Context, svc *MessagingGRPC, acct, profile uuid.UUID, chat *chatv1.ChatRef, content string) *messagingv1.Message {
	t.Helper()
	posted := true
	resp, err := svc.SendMessage(incomingProfileCtx(ctx, acct, profile), &messagingv1.SendMessageRequest{
		Chat:            chat,
		Content:         content,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		PostedAsChat:    &posted,
	})
	require.NoError(t, err)
	return resp.GetMessage()
}

func chatChannelRef(chatID uuid.UUID) *chatv1.ChatRef {
	channel := chatv1.ChatType_CHAT_TYPE_CHANNEL
	return &chatv1.ChatRef{Id: chatID.String(), Type: &channel}
}

func sendRegular(t *testing.T, ctx context.Context, client messagingv1.MessagingServiceClient, acct, profile uuid.UUID, chat *chatv1.ChatRef, content string, threadParentID *string) *messagingv1.Message {
	t.Helper()
	req := &messagingv1.SendMessageRequest{
		Chat:            chat,
		Content:         content,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	}
	if threadParentID != nil {
		req.ThreadParentId = threadParentID
	}
	resp, err := client.SendMessage(withProfileCtx(ctx, acct, profile), req)
	require.NoError(t, err)
	return resp.GetMessage()
}

// TestMessagingThreads_dmReplyExcludedFromMainFeed documents text-chat.md: DM replies stay out of GetMessages.
func TestMessagingThreads_dmReplyExcludedFromMainFeed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	client, _ := startMessagingServer(t, pool)

	parent := sendRegular(t, ctx, client, acctA, profA, chatDMRef(chatID), "parent", nil)
	parentID := parent.GetId()
	sendRegular(t, ctx, client, acctA, profA, chatDMRef(chatID), "reply", &parentID)

	feed, err := client.GetMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatDMRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 20},
	})
	require.NoError(t, err)
	for _, m := range feed.GetMessageList().GetMessages() {
		require.Empty(t, m.GetThreadParentId(), "main feed must exclude thread replies")
	}
	require.Len(t, feed.GetMessageList().GetMessages(), 1, "main feed must contain only root messages")
}

// TestMessagingThreads_getThreadMessagesPaginates documents thread reply history via GetThreadMessages.
func TestMessagingThreads_getThreadMessagesPaginates(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	client, _ := startMessagingServer(t, pool)

	parent := sendRegular(t, ctx, client, acctA, profA, chatDMRef(chatID), "thread root", nil)
	parentID := parent.GetId()
	for i := 0; i < 3; i++ {
		sendRegular(t, ctx, client, acctA, profA, chatDMRef(chatID), "reply", &parentID)
	}

	page1, err := client.GetThreadMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetThreadMessagesRequest{
		Chat:           chatDMRef(chatID),
		ThreadParentId: parentID,
		Page:           &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	ml1 := page1.GetMessageList()
	require.Len(t, ml1.GetMessages(), 2)
	require.True(t, ml1.GetHasMore())
	require.NotEmpty(t, ml1.GetNextCursor())
	for _, m := range ml1.GetMessages() {
		require.Equal(t, parentID, m.GetThreadParentId())
	}

	page2, err := client.GetThreadMessages(withProfileCtx(ctx, acctA, profA), &messagingv1.GetThreadMessagesRequest{
		Chat:           chatDMRef(chatID),
		ThreadParentId: parentID,
		Page:           &commonv1.CursorPageRequest{Cursor: ml1.GetNextCursor(), PageSize: 2},
	})
	require.NoError(t, err)
	ml2 := page2.GetMessageList()
	require.Len(t, ml2.GetMessages(), 1)
	require.False(t, ml2.GetHasMore())
}

// TestMessagingThreads_groupThreadsDisabledRejectsReply documents group default threads_enabled=false.
func TestMessagingThreads_groupThreadsDisabledRejectsReply(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	setChatThreadSettings(t, ctx, pool, chatID, false, true)
	client, _ := startMessagingServer(t, pool)

	parent := sendRegular(t, ctx, client, acctA, profA, chatGroupRef(chatID), "announcement", nil)
	parentID := parent.GetId()

	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "thread reply",
		ThreadParentId:  &parentID,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestMessagingThreads_groupThreadsEnabledAllowsReply documents group threads can be enabled per chat settings.
func TestMessagingThreads_groupThreadsEnabledAllowsReply(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profA, profB)
	setChatThreadSettings(t, ctx, pool, chatID, true, true)
	client, _ := startMessagingServer(t, pool)

	parent := sendRegular(t, ctx, client, acctA, profA, chatGroupRef(chatID), "topic", nil)
	parentID := parent.GetId()

	reply, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatGroupRef(chatID),
		Content:         "discussion",
		ThreadParentId:  &parentID,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	require.Equal(t, parentID, reply.GetMessage().GetThreadParentId())
}

// TestMessagingThreads_channelRejectsUserMainFeed documents channel default allow_user_main_feed=false.
func TestMessagingThreads_channelRejectsUserMainFeed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	acctA := uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profA)
	setChatThreadSettings(t, ctx, pool, chatID, true, false)
	client, _ := startMessagingServer(t, pool)

	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatChannelRef(chatID),
		Content:         "user post",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestMessagingThreads_channelAllowsPostedAsChatMainFeed documents official channel posts in main feed.
func TestMessagingThreads_channelAllowsPostedAsChatMainFeed(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	acctA := uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profA)
	setChatThreadSettings(t, ctx, pool, chatID, true, false)
	svc := startMessagingDirect(t, pool)

	sent, err := svc.SendMessage(incomingProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatChannelRef(chatID),
		Content:         "official update",
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
		PostedAsChat:    ptrBool(true),
	})
	require.NoError(t, err)
	require.True(t, sent.GetMessage().GetPostedAsChat())

	feed, err := svc.GetMessages(incomingProfileCtx(ctx, acctA, profA), &messagingv1.GetMessagesRequest{
		Chat: chatChannelRef(chatID),
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, feed.GetMessageList().GetMessages(), 1)
	require.True(t, feed.GetMessageList().GetMessages()[0].GetPostedAsChat())
}

// TestMessagingThreads_channelAllowsThreadReply documents channel thread replies from members.
func TestMessagingThreads_channelAllowsThreadReply(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	acctA := uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profA)
	setChatThreadSettings(t, ctx, pool, chatID, true, false)
	svc := startMessagingDirect(t, pool)

	parent := sendPostedAsChatDirect(t, ctx, svc, acctA, profA, chatChannelRef(chatID), "news")
	parentID := parent.GetId()

	reply, err := svc.SendMessage(incomingProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatChannelRef(chatID),
		Content:         "question",
		ThreadParentId:  &parentID,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.NoError(t, err)
	require.Equal(t, parentID, reply.GetMessage().GetThreadParentId())
}

// TestMessagingThreads_invalidParentRejected ensures replies target an existing root message in chat.
func TestMessagingThreads_invalidParentRejected(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	client, _ := startMessagingServer(t, pool)

	missing := uuid.New().String()
	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "orphan reply",
		ThreadParentId:  &missing,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

// TestMessagingThreads_nestedParentRejected documents single-level threads (no reply-to-reply).
func TestMessagingThreads_nestedParentRejected(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	acctA := uuid.New()
	seedDMChat(t, ctx, pool, chatID, profA, profB)
	client, _ := startMessagingServer(t, pool)

	parent := sendRegular(t, ctx, client, acctA, profA, chatDMRef(chatID), "root", nil)
	parentID := parent.GetId()
	firstReply := sendRegular(t, ctx, client, acctA, profA, chatDMRef(chatID), "first", &parentID)
	nestedParent := firstReply.GetId()

	_, err := client.SendMessage(withProfileCtx(ctx, acctA, profA), &messagingv1.SendMessageRequest{
		Chat:            chatDMRef(chatID),
		Content:         "nested",
		ThreadParentId:  &nestedParent,
		AttachmentsJson: "[]",
		MentionsJson:    "[]",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
