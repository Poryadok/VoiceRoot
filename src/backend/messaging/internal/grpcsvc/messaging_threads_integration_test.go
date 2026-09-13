package grpcsvc

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	"voice/backend/messaging/internal/store"
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
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000010_ghost_only.up.sql"))
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

	// Equal pre-snapshot times make the UUID tie-breaker observable. Set every
	// existing row into the past so the subsequently inserted rows are truly
	// later in the database-created-time snapshot contract.
	tie := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	_, err := pool.Exec(ctx, `UPDATE messages SET created_at = $2 WHERE chat_id = $1`, chatID, tie)
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

// TestMessagingListThreads_ghostVisibilityContract protects the platform
// moderation boundary for both a root and its reply: a ghost thread remains
// observable to its sender but is absent for another real chat member.
func TestMessagingListThreads_ghostVisibilityContract(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000010_ghost_only.up.sql"))

	chatID, senderProfileID, otherProfileID, accountID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, senderProfileID)
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, otherProfileID)
	require.NoError(t, err)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)

	t.Run("ghost root", func(t *testing.T) {
		parent := sendPostedAsChatDirect(t, ctx, svc, accountID, senderProfileID, chat, "ghost root")
		parentID := parent.GetId()
		_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, senderProfileID), &messagingv1.SendMessageRequest{
			Chat: chat, Content: "visible only to sender", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
		})
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE messages SET ghost_only = true WHERE id = $1`, uuid.MustParse(parentID))
		require.NoError(t, err)
		assertGhostThreadVisibility(t, ctx, svc, accountID, senderProfileID, otherProfileID, chat, parentID)
	})

	t.Run("ghost reply", func(t *testing.T) {
		parent := sendPostedAsChatDirect(t, ctx, svc, accountID, senderProfileID, chat, "normal root")
		parentID := parent.GetId()
		reply, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, senderProfileID), &messagingv1.SendMessageRequest{
			Chat: chat, Content: "ghost reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]",
		})
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `UPDATE messages SET ghost_only = true WHERE id = $1`, uuid.MustParse(reply.GetMessage().GetId()))
		require.NoError(t, err)
		assertGhostThreadVisibility(t, ctx, svc, accountID, senderProfileID, otherProfileID, chat, parentID)
	})
}

func assertGhostThreadVisibility(t *testing.T, ctx context.Context, svc *MessagingGRPC, accountID, senderProfileID, otherProfileID uuid.UUID, chat *chatv1.ChatRef, parentID string) {
	t.Helper()
	self, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, senderProfileID), &messagingv1.ListThreadsRequest{Chat: chat})
	require.NoError(t, err)
	require.Contains(t, threadParentIDs(self.GetThreadList().GetThreads()), parentID)

	other, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, otherProfileID), &messagingv1.ListThreadsRequest{Chat: chat})
	require.NoError(t, err)
	require.NotContains(t, threadParentIDs(other.GetThreadList().GetThreads()), parentID)
}

// TestMessagingListThreads_cursorDoesNotCarryInvisibleMessageID ensures an
// opaque cursor does not disclose a ghost row through its snapshot watermark.
func TestMessagingListThreads_cursorDoesNotCarryInvisibleMessageID(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000010_ghost_only.up.sql"))

	chatID, senderProfileID, otherProfileID, accountID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, senderProfileID)
	_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, otherProfileID)
	require.NoError(t, err)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)
	for range 2 {
		parent := sendPostedAsChatDirect(t, ctx, svc, accountID, senderProfileID, chat, "visible root")
		parentID := parent.GetId()
		_, err = svc.SendMessage(incomingProfileCtx(ctx, accountID, senderProfileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "visible reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]"})
		require.NoError(t, err)
	}
	ghostParent := sendPostedAsChatDirect(t, ctx, svc, accountID, senderProfileID, chat, "ghost root")
	ghostParentID := uuid.MustParse(ghostParent.GetId())
	ghostReply, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, senderProfileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "ghost reply", ThreadParentId: ptrString(ghostParentID.String()), AttachmentsJson: "[]", MentionsJson: "[]"})
	require.NoError(t, err)
	ceilingID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	_, err = pool.Exec(ctx, `UPDATE messages SET id = $1 WHERE id = $2`, ceilingID, ghostParentID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET thread_parent_id = $1, ghost_only = true WHERE id = $2`, ceilingID, uuid.MustParse(ghostReply.GetMessage().GetId()))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET ghost_only = true WHERE id = $1`, ceilingID)
	require.NoError(t, err)

	page, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, otherProfileID), &messagingv1.ListThreadsRequest{Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	cursor, err := verifyThreadCursor([]byte("messaging-thread-cursor-test-secret"), page.GetThreadList().GetNextCursor(), time.Now())
	require.NoError(t, err)
	encoded, err := json.Marshal(cursor)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), ceilingID.String(), "cursor must not carry an invisible message ID")
}

func TestMessagingListThreads_cursorKeepsOriginalExpiry(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)
	for range 3 {
		parent := sendPostedAsChatDirect(t, ctx, svc, accountID, profileID, chat, "root")
		parentID := parent.GetId()
		_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]"})
		require.NoError(t, err)
	}
	first, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	firstCursor, err := verifyThreadCursor([]byte("messaging-thread-cursor-test-secret"), first.GetThreadList().GetNextCursor(), time.Now())
	require.NoError(t, err)
	second, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: chat, Page: &commonv1.CursorPageRequest{Cursor: first.GetThreadList().GetNextCursor(), PageSize: 1}})
	require.NoError(t, err)
	secondCursor, err := verifyThreadCursor([]byte("messaging-thread-cursor-test-secret"), second.GetThreadList().GetNextCursor(), time.Now())
	require.NoError(t, err)
	require.Equal(t, firstCursor.ExpiresAt, secondCursor.ExpiresAt, "later pages must retain the first cursor expiry")
}

func TestMessagingListThreads_cursorSecretIsRequiredAndAtLeast32Bytes(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, profileID)
	chat := chatChannelRef(chatID)
	for _, secret := range [][]byte{nil, []byte(strings.Repeat("x", 31))} {
		svc := startMessagingDirect(t, pool)
		svc.ThreadCursorSecret = secret
		_, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: chat})
		require.Equal(t, codes.FailedPrecondition, status.Code(err), "invalid cursor key length=%d", len(secret))
	}
}

func functionSource(t *testing.T, source, signature string) string {
	t.Helper()
	start := strings.Index(source, signature)
	require.GreaterOrEqual(t, start, 0, "missing function %s", signature)
	bodyStart := strings.Index(source[start:], "{")
	require.GreaterOrEqual(t, bodyStart, 0, "missing function body %s", signature)
	bodyStart += start
	depth := 0
	for i := bodyStart; i < len(source); i++ {
		switch source[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return source[start : i+1]
			}
		}
	}
	t.Fatalf("unterminated function %s", signature)
	return ""
}

// TestThreadSnapshotCeilingUsesCreatedAtHighWater binds the persistence
// contract to both ends of the implementation: the snapshot producer must
// derive created_at, and ListThreads must compare message created_at against it
// rather than admitting rows by UUID.
func TestThreadSnapshotCeilingUsesCreatedAtHighWater(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "messaging", "internal", "store", "messages_store.go"))
	require.NoError(t, err)
	source := string(raw)
	ceiling := functionSource(t, source, "func (s *MessagesStore) ThreadSnapshotCeiling")
	listThreads := functionSource(t, source, "func (s *MessagesStore) ListThreads")
	require.True(t, strings.Contains(ceiling, "MAX(created_at)"), "ThreadSnapshotCeiling must derive the database created_at high-water")
	require.Regexp(t, `m\.created_at\s*<=\s*\$[0-9]+`, listThreads, "ListThreads must apply the created_at snapshot predicate to messages")
	require.NotRegexp(t, `m\.id\s*<=\s*\$[0-9]+`, listThreads, "ListThreads must not use a UUID snapshot ceiling")
}

// TestMessagingListThreads_createdAtSnapshotExcludesLaterLowerUUID is the
// deterministic behavioral companion to the SQL contract. The post-page-one
// thread has a UUID lower than every existing row; only its later database
// creation time may exclude it from the old cursor snapshot.
func TestMessagingListThreads_createdAtSnapshotExcludesLaterLowerUUID(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	chatID, profileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chatID, profileID, uuid.New())
	setChatThreadSettings(t, ctx, pool, chatID, true, true)
	svc := startMessagingDirect(t, pool)
	chat := chatGroupRef(chatID)
	preSnapshotParentIDs := make([]string, 0, 2)
	for range 2 {
		parent := sendRegularDirect(t, ctx, svc, accountID, profileID, chat, "before snapshot")
		parentID := parent.GetId()
		preSnapshotParentIDs = append(preSnapshotParentIDs, parentID)
		_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "before reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]"})
		require.NoError(t, err)
	}
	page1, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	cursor := page1.GetThreadList().GetNextCursor()
	require.NotEmpty(t, cursor)
	pageOneParentID := page1.GetThreadList().GetThreads()[0].GetThreadParentId()

	laterParent := sendRegularDirect(t, ctx, svc, accountID, profileID, chat, "later root with lower UUID")
	laterParentID := uuid.MustParse(laterParent.GetId())
	laterReply, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "later reply with lower UUID", ThreadParentId: ptrString(laterParentID.String()), AttachmentsJson: "[]", MentionsJson: "[]"})
	require.NoError(t, err)
	lowParentID := uuid.MustParse("00000000-0000-4000-8000-000000000001")
	lowReplyID := uuid.MustParse("00000000-0000-4000-8000-000000000002")
	_, err = pool.Exec(ctx, `UPDATE messages SET id = $1 WHERE id = $2`, lowParentID, laterParentID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET id = $1, thread_parent_id = $2 WHERE id = $3`, lowReplyID, lowParentID, uuid.MustParse(laterReply.GetMessage().GetId()))
	require.NoError(t, err)

	page2, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, profileID), &messagingv1.ListThreadsRequest{Chat: chat, Page: &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 1}})
	require.NoError(t, err)
	pageTwoParentIDs := threadParentIDs(page2.GetThreadList().GetThreads())
	require.Len(t, pageTwoParentIDs, 1, "page two must still return the eligible pre-snapshot thread")
	require.NotEqual(t, pageOneParentID, pageTwoParentIDs[0], "page two must not duplicate page one")
	require.Contains(t, preSnapshotParentIDs, pageTwoParentIDs[0], "page two must return a pre-snapshot thread")
	require.NotContains(t, pageTwoParentIDs, lowParentID.String(), "later-created row must stay outside the original created_at snapshot even with a lower UUID")
}

type liveThreadRevocationScene struct {
	chat              *chatv1.ChatRef
	ownerProfileID    uuid.UUID
	otherProfileID    uuid.UUID
	accountID         uuid.UUID
	targetParentID    string
	targetReplyID     string
	pageOneParentID   string
	pageOneNextCursor string
	svc               *MessagingGRPC
	pool              *pgxpool.Pool
}

func newLiveThreadRevocationScene(t *testing.T) liveThreadRevocationScene {
	t.Helper()
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "messaging_db", "000010_ghost_only.up.sql"))
	chatID, ownerProfileID, otherProfileID, accountID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChat(t, ctx, pool, chatID, ownerProfileID, otherProfileID)
	setChatThreadSettings(t, ctx, pool, chatID, true, true)
	svc := startMessagingDirect(t, pool)
	chat := chatGroupRef(chatID)

	parentIDs := make([]string, 0, 3)
	replyByParent := make(map[string]string, 3)
	for i := 0; i < 3; i++ {
		parent := sendRegularDirect(t, ctx, svc, accountID, ownerProfileID, chat, "ordinary root")
		parentID := parent.GetId()
		reply, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, ownerProfileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "ordinary preview", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]"})
		require.NoError(t, err)
		parentIDs = append(parentIDs, parentID)
		replyByParent[parentID] = reply.GetMessage().GetId()
	}
	// Equal times force parent ID as the stable pagination tie-breaker.
	tie := time.Date(2026, time.September, 14, 12, 0, 0, 0, time.UTC)
	_, err := pool.Exec(ctx, `UPDATE messages SET created_at = $2 WHERE chat_id = $1 AND thread_parent_id IS NOT NULL`, chatID, tie)
	require.NoError(t, err)
	sort.Sort(sort.Reverse(sort.StringSlice(parentIDs)))
	_, err = pool.Exec(ctx, `UPDATE messages SET content = 'TARGET-ROOT' WHERE id = $1`, uuid.MustParse(parentIDs[1]))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET content = 'TARGET-PREVIEW' WHERE id = $1`, uuid.MustParse(replyByParent[parentIDs[1]]))
	require.NoError(t, err)
	page1, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, ownerProfileID), &messagingv1.ListThreadsRequest{Chat: chat, Page: &commonv1.CursorPageRequest{PageSize: 1}})
	require.NoError(t, err)
	require.Equal(t, []string{parentIDs[0]}, threadParentIDs(page1.GetThreadList().GetThreads()))
	require.NotEmpty(t, page1.GetThreadList().GetNextCursor())
	return liveThreadRevocationScene{
		chat: chat, ownerProfileID: ownerProfileID, otherProfileID: otherProfileID, accountID: accountID,
		targetParentID: parentIDs[1], targetReplyID: replyByParent[parentIDs[1]], pageOneParentID: parentIDs[0],
		pageOneNextCursor: page1.GetThreadList().GetNextCursor(), svc: svc, pool: pool,
	}
}

func assertPageTwoRevocation(t *testing.T, ctx context.Context, scene liveThreadRevocationScene, viewerProfileID uuid.UUID, cursor string) {
	t.Helper()
	page2, err := scene.svc.ListThreads(incomingProfileCtx(ctx, scene.accountID, viewerProfileID), &messagingv1.ListThreadsRequest{Chat: scene.chat, Page: &commonv1.CursorPageRequest{Cursor: cursor, PageSize: 1}})
	require.NoError(t, err)
	seen := make(map[string]struct{}, len(page2.GetThreadList().GetThreads()))
	for _, thread := range page2.GetThreadList().GetThreads() {
		require.NotEqual(t, scene.targetParentID, thread.GetThreadParentId(), "revoked thread ID must not appear on page two")
		require.NotEqual(t, scene.pageOneParentID, thread.GetThreadParentId(), "page two must not duplicate page one")
		require.NotEqual(t, "TARGET-PREVIEW", thread.GetLastReplyPreview(), "revoked preview must not appear on page two")
		_, duplicate := seen[thread.GetThreadParentId()]
		require.False(t, duplicate, "page two must not contain duplicate thread IDs")
		seen[thread.GetThreadParentId()] = struct{}{}
	}
}

// TestMessagingListThreads_pageTwoReevaluatesLiveRevocations verifies the
// narrow contract: snapshot admission is fixed, but root/reply visibility is
// re-evaluated before every cursor page.
func TestMessagingListThreads_pageTwoReevaluatesLiveRevocations(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name        string
		scope       messagingv1.DeleteScope
		targetReply bool
	}{
		{name: "for everyone root", scope: messagingv1.DeleteScope_DELETE_SCOPE_FOR_EVERYONE},
		{name: "for everyone reply", scope: messagingv1.DeleteScope_DELETE_SCOPE_FOR_EVERYONE, targetReply: true},
		{name: "for me root", scope: messagingv1.DeleteScope_DELETE_SCOPE_FOR_ME},
		{name: "for me reply", scope: messagingv1.DeleteScope_DELETE_SCOPE_FOR_ME, targetReply: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scene := newLiveThreadRevocationScene(t)
			messageID := scene.targetParentID
			if tc.targetReply {
				messageID = scene.targetReplyID
			}
			_, err := scene.svc.DeleteMessage(incomingProfileCtx(ctx, scene.accountID, scene.ownerProfileID), &messagingv1.DeleteMessageRequest{MessageId: messageID, Scope: ptrDeleteScope(tc.scope)})
			require.NoError(t, err)
			assertPageTwoRevocation(t, ctx, scene, scene.ownerProfileID, scene.pageOneNextCursor)
		})
	}

	for _, targetReply := range []bool{false, true} {
		t.Run("ghost "+map[bool]string{false: "root", true: "reply"}[targetReply], func(t *testing.T) {
			scene := newLiveThreadRevocationScene(t)
			// Reissue page one as the other member; the old cursor is bound to its viewer.
			page1, err := scene.svc.ListThreads(incomingProfileCtx(ctx, scene.accountID, scene.otherProfileID), &messagingv1.ListThreadsRequest{Chat: scene.chat, Page: &commonv1.CursorPageRequest{PageSize: 1}})
			require.NoError(t, err)
			messageID := scene.targetParentID
			if targetReply {
				messageID = scene.targetReplyID
			}
			_, err = scene.pool.Exec(ctx, `UPDATE messages SET ghost_only = true WHERE id = $1`, uuid.MustParse(messageID))
			require.NoError(t, err)
			assertPageTwoRevocation(t, ctx, scene, scene.otherProfileID, page1.GetThreadList().GetNextCursor())
			self, err := scene.svc.ListThreads(incomingProfileCtx(ctx, scene.accountID, scene.ownerProfileID), &messagingv1.ListThreadsRequest{Chat: scene.chat})
			require.NoError(t, err)
			require.Contains(t, threadParentIDs(self.GetThreadList().GetThreads()), scene.targetParentID, "ghost sender retains self visibility")
		})
	}
}

type listThreadsFaultGuard struct{ err error }

func (g listThreadsFaultGuard) EnsureMember(context.Context, uuid.UUID, uuid.UUID) error {
	return g.err
}
func (listThreadsFaultGuard) DMOtherProfileID(context.Context, uuid.UUID, uuid.UUID) (uuid.UUID, error) {
	return uuid.Nil, nil
}
func (listThreadsFaultGuard) OtherMemberProfileIDs(context.Context, uuid.UUID, uuid.UUID) ([]uuid.UUID, error) {
	return nil, nil
}
func (listThreadsFaultGuard) MemberRole(context.Context, uuid.UUID, uuid.UUID) (string, error) {
	return "", nil
}

func TestMessagingListThreads_membershipDependencyMapsToUnavailable(t *testing.T) {
	svc := &MessagingGRPC{Messages: &store.MessagesStore{}, ChatGuard: listThreadsFaultGuard{err: errors.New("chat unavailable")}}
	_, err := svc.ListThreads(incomingProfileCtx(context.Background(), uuid.New(), uuid.New()), &messagingv1.ListThreadsRequest{Chat: chatChannelRef(uuid.New())})
	require.Equal(t, codes.Unavailable, status.Code(err))
}

func TestMessagingListThreads_realChatMembershipHasNoHundredMemberSeam(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applyThreadMessagingMigrations(t, ctx, pool)
	chatID, ownerProfileID, accountID := uuid.New(), uuid.New(), uuid.New()
	seedChannelChat(t, ctx, pool, chatID, ownerProfileID)
	var readerProfileID uuid.UUID
	for i := 0; i < 101; i++ {
		profileID := uuid.New()
		_, err := pool.Exec(ctx, `INSERT INTO chat_members (chat_id, profile_id, role) VALUES ($1, $2, 'member')`, chatID, profileID)
		require.NoError(t, err)
		if i == 100 {
			readerProfileID = profileID
		}
	}
	svc := startMessagingDirect(t, pool)
	chat := chatChannelRef(chatID)
	parent := sendPostedAsChatDirect(t, ctx, svc, accountID, ownerProfileID, chat, "root")
	parentID := parent.GetId()
	_, err := svc.SendMessage(incomingProfileCtx(ctx, accountID, ownerProfileID), &messagingv1.SendMessageRequest{Chat: chat, Content: "reply", ThreadParentId: &parentID, AttachmentsJson: "[]", MentionsJson: "[]"})
	require.NoError(t, err)
	listed, err := svc.ListThreads(incomingProfileCtx(ctx, accountID, readerProfileID), &messagingv1.ListThreadsRequest{Chat: chat})
	require.NoError(t, err)
	require.Equal(t, []string{parentID}, threadParentIDs(listed.GetThreadList().GetThreads()))
}

func TestMessagingThreadListIndexMigrationUsesConcurrentCreate(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db", "000016_thread_list_snapshot_index.up.sql"))
	require.NoError(t, err)
	migration := string(raw)
	require.Contains(t, strings.ToUpper(migration), "CREATE INDEX CONCURRENTLY IF NOT EXISTS")
	require.Contains(t, migration, "(chat_id, thread_parent_id, created_at DESC, id DESC)")
	require.Contains(t, migration, "INCLUDE (sender_profile_id, ghost_only)")
	require.Contains(t, migration, "WHERE thread_parent_id IS NOT NULL AND deleted_at IS NULL")
}

func TestMessagingListThreadsUsesPageBoundedCandidatesAndPreviews(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "messaging", "internal", "store", "messages_store.go"))
	require.NoError(t, err)
	source := functionSource(t, string(raw), "func (s *MessagesStore) ListThreads")
	require.Contains(t, source, "candidate_threads AS")
	require.Contains(t, source, "LIMIT $8", "the candidate set must be bounded by the requested page size plus one")
	require.Contains(t, source, "LEFT JOIN LATERAL", "previews must be looked up only for bounded candidates")
	require.Contains(t, source, "LIMIT 1", "each candidate must have at most one preview lookup")
}

func TestMessagingThreadListIndexDoesNotUseVoiceDBMigrationRunner(t *testing.T) {
	root := repoRoot(t)
	runner, err := os.ReadFile(filepath.Join(root, "scripts", "staging", "apply-migrate-jobs.sh"))
	require.NoError(t, err)
	require.NotContains(t, string(runner), "apply_migrate messaging_db", "the concurrent index must not use the transaction-wrapped generic runner")
	require.Contains(t, string(runner), "apply_messaging_thread_list_index")

	docs, err := os.ReadFile(filepath.Join(root, "docs", "DEPLOYMENT.md"))
	require.NoError(t, err)
	deployment := string(docs)
	messagingStart := strings.Index(deployment, "### Messaging `ListThreads` cursor and `messaging_db` index rollout")
	voiceStart := strings.Index(deployment, "### `voice_db` lifecycle migration and readiness")
	require.GreaterOrEqual(t, messagingStart, 0)
	require.Greater(t, voiceStart, messagingStart)
	require.NotContains(t, deployment[voiceStart:], "Messaging thread-list index")
	require.Contains(t, deployment[messagingStart:voiceStart], "voice-migrate-messaging-thread-list-index")
}

func TestMessagingThreadListMigrationRunnerRejectsDirtyState(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "deploy", "templates", "migrate-messaging-thread-list-index-job.yaml"))
	require.NoError(t, err)
	template := string(raw)
	require.Contains(t, template, "SELECT version, dirty FROM schema_migrations")
	require.Contains(t, template, "15:f)")
	require.Contains(t, template, "16:f)")
	require.Contains(t, template, "must be clean at migration version 15 or 16")
	require.NotContains(t, template, "15:t)")
	require.NotContains(t, template, "16:t)")
}

func TestProductionMessagingCursorSecretBootstrapContract(t *testing.T) {
	root := repoRoot(t)
	bootstrap, err := os.ReadFile(filepath.Join(root, "scripts", "prod", "bootstrap-app-secrets.sh"))
	require.NoError(t, err)
	require.Contains(t, string(bootstrap), "--from-literal=MESSAGING_THREAD_CURSOR_HMAC_SECRET=\"$(random_hex 32)\"")

	template, err := os.ReadFile(filepath.Join(root, "deploy", "prod", "secret.example.yaml"))
	require.NoError(t, err)
	require.Contains(t, string(template), "MESSAGING_THREAD_CURSOR_HMAC_SECRET")
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
