package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
)

func TestMessageViewsDeduplicateConcurrentProfilesAndEraseOnDelete(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	chat, profile, id := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type) VALUES($1,$2,'group',$3,'hello','regular')`, id, chat, profile)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.RecordMessageView(ctx, chat, nil, id, profile); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	count, err := s.RecordMessageView(ctx, chat, nil, id, profile)
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	count, err = s.RecordMessageView(ctx, chat, nil, id, uuid.New())
	require.NoError(t, err)
	require.EqualValues(t, 2, count)
	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM read_positions`).Scan(&n))
	require.Zero(t, n, "viewport is independent of private read progress")
	_, err = pool.Exec(ctx, `UPDATE messages SET deleted_at=now() WHERE id=$1`, id)
	require.NoError(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_views WHERE message_id=$1`, id).Scan(&n))
	require.Zero(t, n)
	_, err = s.RecordMessageView(ctx, chat, nil, id, profile)
	require.Error(t, err, "deleted messages cannot recreate views")
}

func TestMessageViewsRejectDMWrongChatAndHiddenMessage(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	chat, profile, id := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type) VALUES($1,$2,'dm',$3,'hello','regular')`, id, chat, profile)
	require.NoError(t, err)
	_, err = s.RecordMessageView(ctx, chat, nil, id, profile)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET chat_type='channel' WHERE id=$1`, id)
	require.NoError(t, err)
	_, err = s.RecordMessageView(ctx, uuid.New(), nil, id, profile)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO message_hides(message_id,profile_id) VALUES($1,$2)`, id, profile)
	require.NoError(t, err)
	_, err = s.RecordMessageView(ctx, chat, nil, id, profile)
	require.Error(t, err)
}

func TestMessageViewsCompactionAndPurgeFence(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	chat, profile, other, id := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,type) VALUES($1,$2,'channel',$3,'hello','regular')`, id, chat, profile)
	require.NoError(t, err)
	_, err = s.RecordMessageView(ctx, chat, nil, id, profile)
	require.NoError(t, err)
	_, err = s.RecordMessageView(ctx, chat, nil, id, other)
	require.NoError(t, err)
	profiles, err := s.MessageViewProfiles(ctx, uuid.Nil, 1)
	require.NoError(t, err)
	require.Len(t, profiles, 1)
	rest, err := s.MessageViewProfiles(ctx, profiles[0], 1)
	require.NoError(t, err)
	require.Len(t, rest, 1)
	require.NotEqual(t, profiles[0], rest[0])
	require.NoError(t, s.DeleteProfileMessageViews(ctx, []uuid.UUID{profile}))
	row, err := s.GetMessageByID(ctx, id)
	require.NoError(t, err)
	require.EqualValues(t, 1, row.ViewCount)
	operation := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO managed_chat_purge_operations(operation_id,chat_id,purge_after,request_sha256,state) VALUES($1,$2,now(),$3,'PENDING')`, operation, chat, make([]byte, 32))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO managed_chat_purge_messages(operation_id,message_id,attachments) VALUES($1,$2,'[]')`, operation, id)
	require.NoError(t, err)
	_, err = s.RecordMessageView(ctx, chat, nil, id, uuid.New())
	require.ErrorIs(t, err, ErrMessageMutationPurging)
	_, err = pool.Exec(ctx, `DELETE FROM messages WHERE id=$1`, id)
	require.NoError(t, err)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_views WHERE message_id=$1`, id).Scan(&count))
	require.Zero(t, count, "hard purge cascades view rows")
}
