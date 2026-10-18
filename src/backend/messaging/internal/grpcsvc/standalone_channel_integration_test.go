package grpcsvc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/chat/testchat"
	"voice/backend/messaging/internal/s2s"
)

// Exercise the production Chat adapter, real NULL space_id and normal channel
// creation. Admin/member rows are authorization fixtures, not a promotion API.
func TestMessagingStandaloneChannelAuthority(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "src/backend/migrations/chat_db/*.up.sql"))
	require.NoError(t, err)
	require.NotEmpty(t, files)
	for _, file := range files {
		sql, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err, "migration %s", file)
	}
	applyBaseMessagingMigrations(t, ctx, pool)

	owner, admin, member, outsider, peer := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	account := uuid.New()
	chatClient, cleanup := testchat.NewBufconnChatClientWith(t, pool, testchat.ChatDeps{
		DeletedAccounts: explicitDeletedAccounts{}, Blocks: testchat.AllowAllBlocks{}, Privacy: testchat.AllowAllPrivacy{},
	})
	t.Cleanup(cleanup)
	name := "Standalone announcements"
	created, err := chatClient.CreateChat(withProfileCtx(ctx, account, owner), &chatv1.CreateChatRequest{
		Type: chatv1.ChatType_CHAT_TYPE_CHANNEL, Name: &name,
	})
	require.NoError(t, err)
	target := uuid.MustParse(created.GetChat().GetId())
	var standalone bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT space_id IS NULL FROM chats WHERE id=$1`, target).Scan(&standalone))
	require.True(t, standalone)
	_, err = pool.Exec(ctx, `INSERT INTO chat_members(chat_id,profile_id,role) VALUES ($1,$2,'admin'),($1,$3,'member')`, target, admin, member)
	require.NoError(t, err)
	source := uuid.New()
	seedGroupChat(t, ctx, pool, source, owner, peer)
	for _, profile := range []uuid.UUID{admin, member, outsider} {
		_, err = pool.Exec(ctx, `INSERT INTO chat_members(chat_id,profile_id,role) VALUES ($1,$2,'member')`, source, profile)
		require.NoError(t, err)
	}
	spy := &spyMessageEvents{}
	client, _ := startMessagingServerWired(t, pool, messagingWire{ChatGuard: s2s.NewGRPCChatGuard(chatClient), MessageEvents: spy})
	original, err := client.SendMessage(withProfileCtx(ctx, account, peer), &messagingv1.SendMessageRequest{
		Chat: chatGroupRef(source), Content: "original payload", AttachmentsJson: "[]", MentionsJson: "[]",
	})
	require.NoError(t, err)
	commentary := "read this"
	counts := func() [2]int {
		var out [2]int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id=$1`, target).Scan(&out[0]))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE chat_id=$1`, target).Scan(&out[1]))
		return out
	}
	forward := func(profile uuid.UUID, ref *chatv1.ChatRef, comment bool, copy bool) (*messagingv1.ForwardMessageResponse, error) {
		req := &messagingv1.ForwardMessageRequest{SourceMessageId: original.GetMessage().GetId(), TargetChat: ref, WithoutAttribution: &copy}
		if comment {
			req.Commentary = &commentary
		}
		return client.ForwardMessage(withProfileCtx(ctx, account, profile), req)
	}
	send := func(profile uuid.UUID, ref *chatv1.ChatRef, official bool) error {
		_, err := client.SendMessage(withProfileCtx(ctx, account, profile), &messagingv1.SendMessageRequest{
			Chat: ref, Content: "direct post", PostedAsChat: &official, AttachmentsJson: "[]", MentionsJson: "[]",
		})
		return err
	}
	assertDenied := func(action func() error) {
		t.Helper()
		before := counts()
		events := spy.eventCount()
		require.Equal(t, codes.PermissionDenied, status.Code(action()))
		require.Equal(t, before, counts(), "denial must not insert commentary, messages or outbox events")
		require.Equal(t, events, spy.eventCount())
	}
	for _, profile := range []uuid.UUID{owner, admin} {
		response, err := forward(profile, chatChannelRef(target), true, false)
		require.NoError(t, err)
		msg := response.GetMessage()
		require.True(t, msg.GetPostedAsChat())
		require.Equal(t, target.String(), msg.GetDisplayChatId())
		require.Equal(t, profile.String(), msg.GetSenderProfileId())
		require.Equal(t, original.GetMessage().GetId(), msg.GetForwardFromId())
		require.Equal(t, "original payload", msg.GetContent())
		one, err := client.GetMessage(withProfileCtx(ctx, account, profile), &messagingv1.GetMessageRequest{MessageId: msg.GetId()})
		require.NoError(t, err)
		require.Equal(t, msg.GetPostedAsChat(), one.GetMessage().GetPostedAsChat())
		require.Equal(t, msg.GetDisplayChatId(), one.GetMessage().GetDisplayChatId())
		assertDenied(func() error { return send(profile, chatChannelRef(target), false) })
		require.NoError(t, send(profile, chatChannelRef(target), true))
	}
	for _, ref := range []*chatv1.ChatRef{chatChannelRef(target), chatGroupRef(target), chatDMRef(target)} {
		for _, comment := range []bool{false, true} {
			for _, copy := range []bool{false, true} {
				assertDenied(func() error { _, err := forward(member, ref, comment, copy); return err })
			}
		}
		assertDenied(func() error { return send(member, ref, true) })
	}
	assertDenied(func() error { _, err := forward(outsider, chatChannelRef(target), true, false); return err })
	history, err := client.GetMessages(withProfileCtx(ctx, account, owner), &messagingv1.GetMessagesRequest{
		Chat: chatChannelRef(target), Page: &commonv1.CursorPageRequest{PageSize: 100},
	})
	require.NoError(t, err)
	for _, msg := range history.GetMessageList().GetMessages() {
		require.True(t, msg.GetPostedAsChat(), "commentary and forward share the authorized official identity")
		require.Equal(t, target.String(), msg.GetDisplayChatId())
	}
	parentID := history.GetMessageList().GetMessages()[0].GetId()
	threadSend := func(profile uuid.UUID, ref *chatv1.ChatRef, official bool) (*messagingv1.SendMessageResponse, error) {
		return client.SendMessage(withProfileCtx(ctx, account, profile), &messagingv1.SendMessageRequest{
			Chat: ref, ThreadParentId: &parentID, PostedAsChat: &official,
			Content: "thread reply", AttachmentsJson: "[]", MentionsJson: "[]",
		})
	}
	for _, ref := range []*chatv1.ChatRef{chatChannelRef(target), chatGroupRef(target), chatDMRef(target)} {
		assertDenied(func() error { _, err := threadSend(member, ref, true); return err })
	}
	for _, profile := range []uuid.UUID{owner, admin} {
		reply, err := threadSend(profile, chatChannelRef(target), true)
		require.NoError(t, err)
		require.True(t, reply.GetMessage().GetPostedAsChat())
		require.Equal(t, target.String(), reply.GetMessage().GetDisplayChatId())
		require.Equal(t, profile.String(), reply.GetMessage().GetSenderProfileId())
	}
	reply, err := threadSend(member, chatChannelRef(target), false)
	require.NoError(t, err, "ordinary thread replies remain available while user main-feed posts are disabled")
	require.False(t, reply.GetMessage().GetPostedAsChat())
	require.Empty(t, reply.GetMessage().GetDisplayChatId())
	allow := true
	_, err = chatClient.UpdateChat(withProfileCtx(ctx, account, member), &chatv1.UpdateChatRequest{ChatId: target.String(), AllowUserMainFeed: &allow})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = chatClient.UpdateChat(withProfileCtx(ctx, account, owner), &chatv1.UpdateChatRequest{ChatId: target.String(), AllowUserMainFeed: &allow})
	require.NoError(t, err)
	for _, profile := range []uuid.UUID{owner, admin, member} {
		response, err := forward(profile, chatChannelRef(target), true, false)
		require.NoError(t, err)
		require.False(t, response.GetMessage().GetPostedAsChat())
		require.Empty(t, response.GetMessage().GetDisplayChatId())
		require.NoError(t, send(profile, chatChannelRef(target), false))
	}
	assertDenied(func() error { return send(member, chatChannelRef(target), true) })
	assertDenied(func() error { _, err := threadSend(member, chatChannelRef(target), true); return err })
	for _, profile := range []uuid.UUID{owner, admin} {
		require.NoError(t, send(profile, chatChannelRef(target), true))
	}
	allow = false
	_, err = chatClient.UpdateChat(withProfileCtx(ctx, account, admin), &chatv1.UpdateChatRequest{ChatId: target.String(), AllowUserMainFeed: &allow})
	require.NoError(t, err)
	assertDenied(func() error { _, err := forward(member, chatChannelRef(target), true, false); return err })
	response, err := forward(owner, chatChannelRef(target), true, false)
	require.NoError(t, err)
	require.True(t, response.GetMessage().GetPostedAsChat())
	_, err = pool.Exec(ctx, `UPDATE chat_members SET role='member' WHERE chat_id=$1 AND profile_id=$2`, target, admin)
	require.NoError(t, err)
	assertDenied(func() error { _, err := forward(admin, chatChannelRef(target), true, false); return err })
	assertDenied(func() error { _, err := threadSend(admin, chatChannelRef(target), true); return err })
	_, err = pool.Exec(ctx, `DELETE FROM chat_members WHERE chat_id=$1 AND profile_id=$2`, target, member)
	require.NoError(t, err)
	assertDenied(func() error { _, err := forward(member, chatChannelRef(target), true, false); return err })
}
