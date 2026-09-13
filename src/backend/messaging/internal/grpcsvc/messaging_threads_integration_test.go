package grpcsvc

import (
	"context"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
)

// applyThreadMessagingMigrations applies chat + messaging schemas used by roles/threads (docs/features/roles.md) thread tests.
func applyThreadMessagingMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000005_thread_settings.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000002_client_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000004_delete_for_me.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000011_last_delivered_message_id.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000012_messages_content_type.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000007_thread_index.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000016_thread_list_snapshot_index.up.sql"))
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

func sendRegularDirect(t *testing.T, ctx context.Context, svc *MessagingGRPC, acct, profile uuid.UUID, chat *chatv1.ChatRef, content string) *messagingv1.Message {
	t.Helper()
	resp, err := svc.SendMessage(incomingProfileCtx(ctx, acct, profile), &messagingv1.SendMessageRequest{
		Chat: chat, Content: content, AttachmentsJson: "[]", MentionsJson: "[]",
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

// TestMessagingListThreads_cursorPaginationSnapshotContract captures the accepted
// ListThreads v1 cursor contract: it is a signed, chat/profile/page-size-bound
// snapshot cursor and pages descending by (last_reply_at, thread_parent_id).
func TestMessagingListThreads_cursorPaginationSnapshotContract(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)

	parentIDs := make([]string, 0, 4)
	for i := 0; i < 4; i++ {
		parent := sendPostedAsChatDirect(t, ctx, svc, accountID, profileID, chat, "root")
		parentID := parent.GetId()
		parentIDs = append(parentIDs, parentID)
		threadReplyReq := &messagingv1.SendMessageRequest{
			Chat: chat, Content: "reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
		}
		_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), threadReplyReq)
		require.NoError(t, err)
	}

	// Equal reply times make the UUID tie-breaker observable.
	tie := time.Date(2026, time.September, 14, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `UPDATE messages SET created_at = $2 WHERE chat_id = $1 AND thread_parent_id IS NOT NULL`, chatID, tie)
	require.NoError(t, err)
	sort.Sort(sort.Reverse(sort.StringSlice(parentIDs)))

	page1, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{
		Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	require.Len(t, page1.GetThreadList().GetThreads(), 2)
	require.Equal(t, parentIDs[:2], threadParentIDs(page1.GetThreadList().GetThreads()))
	cursor := page1.GetThreadList().GetNextCursor()
	require.NotEmpty(t, cursor, "N+1 lookahead must issue a cursor when another visible thread exists")

	// A reply created after the first page belongs after its snapshot ceiling.
	newParent := sendPostedAsChatDirect(t, ctx, svc, accountID, profileID, chat, "new root")
	newParentID := newParent.GetId()
	_, err = svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
		Chat: chat, Content: "new reply", ThreadParentId: &newParentID, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	// A new reply on an item already returned on page one must not duplicate it on page two.
	updatedParentID := parentIDs[0]
	_, err = svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
		Chat: chat, Content: "updated reply", ThreadParentId: &updatedParentID, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)

	page2, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{
		Chat: chat, Page: &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 2},
	})
	require.NoError(t, err)
	require.Equal(t, parentIDs[2:], threadParentIDs(page2.GetThreadList().GetThreads()))
	require.Empty(t, page2.GetThreadList().GetNextCursor())
}

// TestMessagingListThreads_cursorValidationAndMembershipPrecedence fixes the
// public error contract: access is checked before decoding a cursor, while a
// malformed or differently-bound cursor is never accepted.
func TestMessagingListThreads_cursorValidationAndMembershipPrecedence(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)

	t.Run("forged cursor", func(t *testing.T) {
		_, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{
			Chat: chat, Page: &commonv1.CursorPageRequest{Cursor: "forged-v1-cursor", PageSize: 2},
		})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("revoked membership precedes cursor validation", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM chat_members WHERE chat_id = $1 AND profile_id = $2`, chatID, profileID)
		require.NoError(t, err)
		_, err = svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{
			Chat: chat, Page: &commonv1.CursorPageRequest{Cursor: "forged-v1-cursor", PageSize: 2},
		})
		require.Equal(t, codes.PermissionDenied, status.Code(err), "revoked membership takes precedence over cursor validation")
	})
}

// TestMessagingListThreads_cursorBindingContract fixes every request identity
// that a v1 HMAC cursor binds: RPC, chat, authenticated profile and effective
// page size. The accepted Sol decision keeps page_size>100 invalid rather than
// silently clamping it.
func TestMessagingListThreads_cursorBindingContract(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)
	for i := 0; i < 3; i++ {
		parent := sendPostedAsChatDirect(t, ctx, svc, accountID, profileID, chat, "root")
		parentID := parent.GetId()
		_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
			Chat: chat, Content: "reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
		})
		require.NoError(t, err)
	}
	page1, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{
		Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 2},
	})
	require.NoError(t, err)
	cursor := page1.GetThreadList().GetNextCursor()
	require.NotEmpty(t, cursor, "first page must provide a signed cursor for binding checks")

	otherChatID := uuid.New()
	seedChannelChat(t, ctx, pool, otherChatID, profileID)
	otherChat := chatChannelRef(otherChatID)
	otherProfileID := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, otherProfileID)
	require.NoError(t, err)

	for _, tc := range []struct {
		name    string
		chat    *chatv1.ChatRef
		profile uuid.UUID
		size    int32
	}{
		{name: "cross chat", chat: otherChat, profile: profileID, size: 2},
		{name: "cross profile", chat: chat, profile: otherProfileID, size: 2},
		{name: "changed page size", chat: chat, profile: profileID, size: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, tc.profile), &messagingv1.ListThreadsRequest{
				Chat: tc.chat, Page: &commonv1.CursorPageRequest{Cursor: cursor, PageSize: tc.size},
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestMessagingListThreads_pageSizeContract(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)

	for _, size := range []int32{-1, 101} {
		t.Run("invalid page size", func(t *testing.T) {
			_, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{
				Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: size},
			})
			require.Equal(t, codes.InvalidArgument, status.Code(err), "page_size=%d", size)
		})
	}
}

func TestMessagingListThreads_viewerPrivacyContract(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	chatID, profileID, otherProfileID, accountID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, otherProfileID)
	require.NoError(t, err)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)

	parent := sendPostedAsChatDirect(t, ctx, svc, accountID, profileID, chat, "private root")
	parentID := parent.GetId()
	reply, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
		Chat: chat, Content: "private reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	for _, messageID := range []string{parentID, reply.GetMessage().GetId()} {
		_, err = svc.DeleteMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.DeleteMessageRequest{
			MessageId: messageID, Scope: ptrDeleteScope(messagingv1.DeleteScope_DELETE_SCOPE_FOR_ME),
		})
		require.NoError(t, err)
	}

	t.Run("hiding viewer", func(t *testing.T) {
		visible, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: chat})
		require.NoError(t, err)
		require.Empty(t, visible.GetThreadList().GetThreads(), "a hidden root or all hidden replies must disappear immediately for that viewer")
	})
	t.Run("other authorized viewer", func(t *testing.T) {
		visible, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, otherProfileID), &messagingv1.ListThreadsRequest{Chat: chat})
		require.NoError(t, err)
		require.Equal(t, []string{parentID}, threadParentIDs(visible.GetThreadList().GetThreads()))
	})
}

func TestMessagingListThreads_normalReadAccessAcrossChatTypes(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)

	profileID, accountID := uuid.New(), uuid.New()
	svc := startMessagingDirect(t, pool)
	for _, tc := range []struct {
		name string
		chat *chatv1.ChatRef
		seed func(uuid.UUID)
	}{
		{
			name: "DM", chat: chatDMRef(uuid.New()),
			seed: func(chatID uuid.UUID) { seedDMChat(t, ctx, pool, chatID, profileID, uuid.New()) },
		},
		{
			name: "group", chat: chatGroupRef(uuid.New()),
			seed: func(chatID uuid.UUID) {
				seedGroupChat(t, ctx, pool, chatID, profileID, uuid.New())
				setChatThreadSettings(t, ctx, pool, chatID, true, true)
			},
		},
		{
			name: "channel", chat: chatChannelRef(uuid.New()),
			seed: func(chatID uuid.UUID) { seedChannelChat(t, ctx, pool, chatID, profileID) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.seed(uuid.MustParse(tc.chat.GetId()))
			if tc.name == "DM" {
				client, _ := startMessagingServer(t, pool)
				parent := sendRegular(t, ctx, client, accountID, profileID, tc.chat, "root", nil)
				parentID := parent.GetId()
				sendRegular(t, ctx, client, accountID, profileID, tc.chat, "reply", &parentID)
				listed, err := client.ListThreads(withProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: tc.chat})
				require.NoError(t, err)
				require.Equal(t, []string{parentID}, threadParentIDs(listed.GetThreadList().GetThreads()))
				return
			}

			var parentID string
			if tc.name == "channel" {
				parentID = sendPostedAsChatDirect(t, ctx, svc, accountID, profileID, tc.chat, "root").GetId()
			} else {
				parentID = sendRegularDirect(t, ctx, svc, accountID, profileID, tc.chat, "root").GetId()
			}
			_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{
				Chat: tc.chat, Content: "reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
			})
			require.NoError(t, err)
			listed, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: tc.chat})
			require.NoError(t, err)
			require.Equal(t, []string{parentID}, threadParentIDs(listed.GetThreadList().GetThreads()))
		})
	}
}

func threadParentIDs(threads []*messagingv1.ThreadSummary) []string {
	ids := make([]string, 0, len(threads))
	for _, thread := range threads {
		ids = append(ids, thread.GetThreadParentId())
	}
	return ids
}

func ptrDeleteScope(scope messagingv1.DeleteScope) *messagingv1.DeleteScope { return &scope }
