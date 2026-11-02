package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	chatv1 "voice.app/voice/chat/v1"
	commonv1 "voice.app/voice/common/v1"
)

// UUID allocation can precede the insertion transaction by arbitrary time,
// including network/file-reference work or a process pause on another replica.
func TestHistoryCursorRecoversLateCommitWithEarlierUUID(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	chat, sender := uuid.New(), uuid.New()
	a := uuid.MustParse("019a0000-0000-7000-8000-000000000001")
	b := uuid.MustParse("019a0000-0000-7000-8000-000000000002")
	row := func(id uuid.UUID) MessageRow {
		return MessageRow{ID: id, ChatID: chat, SenderProfileID: sender,
			Content: "message", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]"}
	}
	// A has an ID and an open transaction, but has not reached insertion.
	txA, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer txA.Rollback(ctx)
	_, err = s.InsertMessage(ctx, row(b))
	require.NoError(t, err)
	first, err := s.ListMessages(ctx, chat, sender, ListLatest, nil, 1)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, b, first[0].ID)
	_, err = insertMessageDB(ctx, txA, row(a))
	require.NoError(t, err)
	require.NoError(t, txA.Commit(ctx))
	// No live event is delivered. The persisted B cursor must recover A.
	recovered, err := s.ListMessages(ctx, chat, sender, ListAfterID, &b, 1)
	require.NoError(t, err)
	require.Len(t, recovered, 1, "catch-up must not skip a later commit with a smaller UUID")
	require.Equal(t, a, recovered[0].ID)
	latest, err := s.ListMessages(ctx, chat, sender, ListLatest, nil, 1)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{a, b}, historyIDs(latest))
	older, err := s.ListMessages(ctx, chat, sender, ListBeforeID, &a, 1)
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{b}, historyIDs(older))
	end, err := s.ListMessages(ctx, chat, sender, ListAfterID, &a, 1)
	require.NoError(t, err)
	require.Empty(t, end)
}

func historyIDs(rows []MessageRow) []uuid.UUID {
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	return ids
}

func TestHistoryPositionSerializesSameChatInsertionThroughCommit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	chat, sender := uuid.New(), uuid.New()
	a, b := uuid.New(), uuid.New()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background())
	insert := `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content) VALUES($1,$2,'dm',$3,'message')`
	_, err = tx.Exec(ctx, insert, a, chat, sender)
	require.NoError(t, err)
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	pid := conn.Conn().PgConn().PID()
	done := make(chan error, 1)
	go func() {
		defer close(done)
		_, err := conn.Exec(ctx, insert, b, chat, sender)
		done <- err
	}()
	defer func() {
		_ = tx.Rollback(context.Background())
		cancel()
		<-done
		conn.Release()
	}()
	// Observe the actual lock wait, rather than rely on a scheduling sleep.
	require.Eventually(t, func() bool {
		var blocked bool
		err := pool.QueryRow(ctx, `SELECT cardinality(pg_blocking_pids($1))>0`, pid).Scan(&blocked)
		return err == nil && blocked
	}, 10*time.Second, 10*time.Millisecond)
	// A different chat must progress while this chat's transaction is open.
	_, err = pool.Exec(ctx, insert, uuid.New(), uuid.New(), sender)
	require.NoError(t, err)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	var aPosition, bPosition int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_position FROM messages WHERE id=$1`, a).Scan(&aPosition))
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_position FROM messages WHERE id=$1`, b).Scan(&bPosition))
	require.Greater(t, bPosition, aPosition)
	_, err = pool.Exec(ctx, `UPDATE messages SET history_position=history_position+100 WHERE id=$1`, a)
	require.ErrorContains(t, err, "message history position is immutable")
	_, err = pool.Exec(ctx, `UPDATE messages SET chat_id=$2 WHERE id=$1`, a, uuid.New())
	require.ErrorContains(t, err, "message history position is immutable")
}

func TestHistoryPositionMigrationBackfillAndRollbackPreserveMessages(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000001_init.up.sql")
	a := uuid.MustParse("019a0000-0000-7000-8000-000000000001")
	b := uuid.MustParse("019a0000-0000-7000-8000-000000000002")
	chat, sender := uuid.New(), uuid.New()
	insert := `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content) VALUES($1,$2,'dm',$3,'legacy')`
	_, err := pool.Exec(ctx, insert, b, chat, sender)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, insert, a, chat, sender)
	require.NoError(t, err)
	apply := func(suffix string) {
		sql, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/messaging_db/000039_history_commit_order."+suffix+".sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sql))
		require.NoError(t, err)
	}
	apply("up")
	var aPosition, bPosition int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_position FROM messages WHERE id=$1`, a).Scan(&aPosition))
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_position FROM messages WHERE id=$1`, b).Scan(&bPosition))
	require.Less(t, aPosition, bPosition)
	c := uuid.New()
	_, err = pool.Exec(ctx, insert, c, chat, sender)
	require.NoError(t, err)
	var cPosition int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT history_position FROM messages WHERE id=$1`, c).Scan(&cPosition))
	require.Greater(t, cPosition, bPosition)
	apply("down")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE chat_id=$1 AND id=ANY($2) AND content='legacy'`, chat, []uuid.UUID{a, b, c}).Scan(&count))
	require.Equal(t, 3, count)
	apply("up")
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE history_position IS NOT NULL`).Scan(&count))
	require.Equal(t, 3, count)
}

func TestHistoryPositionMigrationPreservesFrozenSpaceFence(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000039_history_commit_order.down.sql")
	space, operation, chat := uuid.New(), uuid.New(), uuid.New()
	manifest := &commonv1.ManifestBinding{ManifestId: uuid.NewString(), ItemCount: 1}
	manifest.ManifestSha256 = messagingChatManifestSHA(space, operation, 7, []uuid.UUID{chat})
	page := &chatv1.SpacePurgeManifestPage{ProtocolVersion: 1, Manifest: proto.Clone(manifest).(*commonv1.ManifestBinding), ItemIds: []string{chat.String()}, PageSha256: make([]byte, 32)}
	s := &MessagesStore{Pool: pool}
	_, err := s.ImportSpacePurgeManifestPage(ctx, SpacePurgeManifestPageInput{
		SpaceID: space, DeletionOperationID: operation, ScheduleGeneration: 7, Page: page, SealsManifest: true,
		RequestBytes: []byte("manifest"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("manifest-receipt"),
	})
	require.NoError(t, err)
	id := uuid.New()
	_, err = s.InsertMessage(ctx, MessageRow{ID: id, ChatID: chat, ChatType: "group", SenderProfileID: uuid.New(), Content: "retained", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]"})
	require.NoError(t, err)
	_, err = s.ApplySpaceLifecycleFence(ctx, SpaceLifecycleFenceInput{
		SpaceID: space, DeletionOperationID: operation, Generation: 7,
		State: commonv1.LifecycleFenceState_LIFECYCLE_FENCE_STATE_FROZEN, Manifest: manifest,
		RequestBytes: []byte("freeze"), RequestSHA256: make([]byte, 32), ReceiptBytes: []byte("freeze-receipt"),
	})
	require.NoError(t, err)
	applySQLFile(t, ctx, pool, "src/backend/migrations/messaging_db/000039_history_commit_order.up.sql")
	var content string
	var position int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT content,history_position FROM messages WHERE id=$1`, id).Scan(&content, &position))
	require.Equal(t, "retained", content)
	require.Positive(t, position)
	_, err = pool.Exec(ctx, `UPDATE messages SET content='changed' WHERE id=$1`, id)
	require.ErrorContains(t, err, "Space lifecycle fence")
	var enabled string
	require.NoError(t, pool.QueryRow(ctx, `SELECT tgenabled::text FROM pg_trigger WHERE tgrelid='messages'::regclass AND tgname='messaging_space_chat_mutation_gate'`).Scan(&enabled))
	require.Equal(t, "O", enabled)
}
