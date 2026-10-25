package grpcsvc

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"path/filepath"
	"testing"
	"time"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/messaging/internal/store"
)

func TestMessageViewsViewportReadCursorAndHistory(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	chat, owner, viewer, other := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChatWithMembersForReadState(t, ctx, pool, chat, owner, viewer, other)
	client, cleanup := startMessagingServer(t, pool)
	defer cleanup()
	ownerCtx := withProfileCtx(ctx, uuid.New(), owner)
	viewerCtx := withProfileCtx(ctx, uuid.New(), viewer)
	first, err := client.SendMessage(ownerCtx, &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "first"})
	require.NoError(t, err)
	second, err := client.SendMessage(ownerCtx, &messagingv1.SendMessageRequest{Chat: chatGroupRef(chat), Content: "second"})
	require.NoError(t, err)
	req := &messagingv1.RecordMessageViewRequest{ChatId: chat.String(), MessageId: first.Message.Id}
	for i := 0; i < 2; i++ {
		r, err := client.RecordMessageView(viewerCtx, req)
		require.NoError(t, err)
		require.EqualValues(t, 1, r.ViewCount)
	}
	_, err = client.MarkRead(viewerCtx, &messagingv1.MarkReadRequest{Chat: chatGroupRef(chat), LastReadMessageId: second.Message.Id})
	require.NoError(t, err)
	for _, id := range []string{first.Message.Id, second.Message.Id} {
		r, err := client.GetMessage(ownerCtx, &messagingv1.GetMessageRequest{MessageId: id})
		require.NoError(t, err)
		require.NotNil(t, r.Message.ViewCount)
		require.EqualValues(t, 1, r.Message.GetViewCount())
	}
	edited, err := client.EditMessage(ownerCtx, &messagingv1.EditMessageRequest{MessageId: first.Message.Id, Content: "edited"})
	require.NoError(t, err)
	require.NotNil(t, edited.Message.ViewCount)
	require.EqualValues(t, 1, edited.Message.GetViewCount(), "editing cannot reset durable views")
	_, err = client.RecordMessageView(withProfileCtx(ctx, uuid.New(), uuid.New()), req)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	_, err = client.RecordMessageView(viewerCtx, &messagingv1.RecordMessageViewRequest{ChatId: uuid.NewString(), MessageId: first.Message.Id})
	require.Error(t, err)
}

func TestMessageViewsReadRangePagesAndFiltersVisibility(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForTest(t, ctx)
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000001_init.up.sql"))
	applySQLFile(t, ctx, pool, filepath.Join("src", "backend", "migrations", "chat_db", "000003_groups.up.sql"))
	applyBaseMessagingMigrations(t, ctx, pool)
	chat, owner, viewer, blocked := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	seedGroupChatWithMembersForReadState(t, ctx, pool, chat, owner, viewer, blocked)
	cutoff := time.Now().UTC().Add(-time.Hour)
	guard := &cutoffMessageReadGuard{entitledSQLChatGuard: entitledSQLChatGuard{SQLChatGuard: &store.SQLChatGuard{Pool: pool}}, cutoff: cutoff}
	client, cleanup := startMessagingServerWired(t, pool, messagingWire{ChatGuard: guard, ProfilePairBlocks: directionalProfilePairBlocks{{viewer, blocked}: true}})
	defer cleanup()
	ids := make([]uuid.UUID, 205)
	for i := range ids {
		ids[i] = uuid.Must(uuid.NewV7())
		_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type) VALUES($1,$2,'group',$3,'visible','regular')`, ids[i], chat, owner)
		require.NoError(t, err)
	}
	excluded := []uuid.UUID{uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())}
	for i, id := range excluded {
		sender := owner
		if i == 0 {
			sender = blocked
		}
		_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type,ghost_only,created_at) VALUES($1,$2,'group',$3,'hidden','regular',$4,$5)`, id, chat, sender, i == 1, cutoff.Add(time.Hour))
		require.NoError(t, err)
	}
	_, err := pool.Exec(ctx, `INSERT INTO message_hides(message_id,profile_id) VALUES($1,$2)`, excluded[2], viewer)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET created_at=$2 WHERE id=$1`, excluded[3], cutoff.Add(-time.Minute))
	require.NoError(t, err)
	target := uuid.Must(uuid.NewV7())
	_, err = pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type) VALUES($1,$2,'group',$3,'target','regular')`, target, chat, owner)
	require.NoError(t, err)
	viewerCtx := withProfileCtx(ctx, uuid.New(), viewer)
	for _, id := range excluded {
		_, err := client.RecordMessageView(viewerCtx, &messagingv1.RecordMessageViewRequest{ChatId: chat.String(), MessageId: id.String()})
		require.Equal(t, codes.NotFound, status.Code(err))
	}
	_, err = client.MarkRead(viewerCtx, &messagingv1.MarkReadRequest{Chat: chatGroupRef(chat), LastReadMessageId: target.String()})
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_views WHERE profile_id=$1`, viewer).Scan(&count))
	require.Equal(t, 206, count)
	_, err = client.MarkRead(viewerCtx, &messagingv1.MarkReadRequest{Chat: chatGroupRef(chat), LastReadMessageId: target.String()})
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_views WHERE profile_id=$1`, viewer).Scan(&count))
	require.Equal(t, 206, count)
}
