package grpcsvc

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	commonv1 "voice.app/voice/common/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

// The Flutter regression uses this same wire contract: latest B, then pages of
// late commits whose timestamp and UUID both precede B's display position.
func TestMessagingGetMessages_LateCommitPagesAdvanceWithEarlierUUIDs(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
	applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000014_managed_chats.up.sql")
	applyBaseMessagingMigrations(t, ctx, pool)
	chat, sender, peer, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedDMChat(t, ctx, pool, chat, sender, peer)
	client, _ := startMessagingServer(t, pool)
	call := withProfileCtx(ctx, account, sender)
	b := "019affff-ffff-7000-8000-000000000001"
	insert := `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,created_at) VALUES($1,$2,'dm',$3,'message',$4::text::timestamptz)`
	_, err := pool.Exec(ctx, insert, b, chat, sender, "2026-01-01T00:00:00Z")
	require.NoError(t, err)
	initial, err := client.GetMessages(call, &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat)})
	require.NoError(t, err)
	require.Equal(t, b, initial.GetMessageList().GetMessages()[0].GetId())
	want := make([]string, 101)
	for i := range want {
		want[i] = fmt.Sprintf("019a0000-0000-7000-8000-%012d", 999998-i)
		_, err = pool.Exec(ctx, insert, want[i], chat, sender, "2024-01-01T00:00:00Z")
		require.NoError(t, err)
	}
	request := &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat), AfterMessageId: &b, Page: &commonv1.CursorPageRequest{PageSize: 50}}
	var got []string
	var pageSizes []int
	for pageNumber := 0; ; pageNumber++ {
		require.Less(t, pageNumber, 4, "cursor chain must make bounded progress")
		response, err := client.GetMessages(call, request)
		require.NoError(t, err)
		page := response.GetMessageList()
		pageSizes = append(pageSizes, len(page.GetMessages()))
		for _, message := range page.GetMessages() {
			got = append(got, message.GetId())
		}
		if !page.GetHasMore() {
			break
		}
		require.NotEmpty(t, page.GetNextCursor())
		require.NotEqual(t, request.GetPage().GetCursor(), page.GetNextCursor())
		request = &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat), Page: &commonv1.CursorPageRequest{PageSize: 50, Cursor: page.GetNextCursor()}}
	}
	require.Equal(t, []int{50, 50, 1}, pageSizes)
	require.Equal(t, want, got, "commit order must survive the full public cursor chain")
	end, err := client.GetMessages(call, &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat), LastMessageId: &got[len(got)-1]})
	require.NoError(t, err)
	require.Empty(t, end.GetMessageList().GetMessages())
}

func TestMessagingGetMessages_MissingAnchorSignalsLatestReset(t *testing.T) {
	for _, count := range []int{5, 101} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			ctx := context.Background()
			pool := startPostgresForTest(t, ctx)
			applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000001_init.up.sql")
			applySQLFile(t, ctx, pool, "src/backend/migrations/chat_db/000014_managed_chats.up.sql")
			applyBaseMessagingMigrations(t, ctx, pool)
			chat, sender, peer, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
			seedDMChat(t, ctx, pool, chat, sender, peer)
			client, _ := startMessagingServer(t, pool)
			call := withProfileCtx(ctx, account, sender)
			b := "019affff-ffff-7000-8000-000000000001"
			insert := `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,created_at) VALUES($1,$2,'dm',$3,'message',$4::text::timestamptz)`
			_, err := pool.Exec(ctx, insert, b, chat, sender, "2026-01-01T00:00:00Z")
			require.NoError(t, err)
			ids := make([]string, count)
			for i := range ids {
				ids[i] = fmt.Sprintf("019a0000-0000-7000-8000-%012d", 999998-i)
				_, err = pool.Exec(ctx, insert, ids[i], chat, sender, "2024-01-01T00:00:00Z")
				require.NoError(t, err)
			}
			first, err := client.GetMessages(call, &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat), AfterMessageId: &b, Page: &commonv1.CursorPageRequest{PageSize: 50}})
			require.NoError(t, err)
			require.False(t, first.GetHistoryReset())
			_, err = pool.Exec(ctx, `UPDATE messages SET deleted_at=clock_timestamp() WHERE id=$1`, b)
			require.NoError(t, err)
			reset, err := client.GetMessages(call, &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat), AfterMessageId: &b})
			require.NoError(t, err)
			require.True(t, reset.GetHistoryReset(), "deleted explicit anchor is a latest snapshot, not a forward page")
			wantCount := count
			if wantCount > 50 {
				wantCount = 50
			}
			require.Len(t, reset.GetMessageList().GetMessages(), wantCount)
			require.Equal(t, ids[len(ids)-1], reset.GetMessageList().GetMessages()[0].GetId())
			require.Equal(t, count > 50, reset.GetMessageList().GetHasMore())
			if count <= 50 {
				require.Empty(t, reset.GetMessageList().GetNextCursor())
				return
			}
			before, after, err := store.DecodeHistoryCursor(reset.GetMessageList().GetNextCursor(), chat)
			require.NoError(t, err)
			require.NotNil(t, before)
			require.Nil(t, after)
			_, err = pool.Exec(ctx, `UPDATE messages SET deleted_at=clock_timestamp() WHERE id=$1`, ids[49])
			require.NoError(t, err)
			continuation, err := client.GetMessages(call, &messagingv1.GetMessagesRequest{Chat: chatDMRef(chat), Page: &commonv1.CursorPageRequest{Cursor: first.GetMessageList().GetNextCursor()}})
			require.NoError(t, err)
			require.True(t, continuation.GetHistoryReset(), "deleted opaque continuation anchor also resets")
			require.Equal(t, ids[len(ids)-1], continuation.GetMessageList().GetMessages()[0].GetId())
		})
	}
}
