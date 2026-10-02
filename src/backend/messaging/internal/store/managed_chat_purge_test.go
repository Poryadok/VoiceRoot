package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestManagedChatPurgeFreezesCutoffSetAndReplaysAfterMessageChanges(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	chatID, senderID := uuid.New(), uuid.New()
	cutoff := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	beforeID, equalID, afterID := uuid.New(), uuid.New(), uuid.New()
	for _, item := range []struct {
		id      uuid.UUID
		created time.Time
		file    string
	}{
		{beforeID, cutoff.Add(-time.Second), uuid.NewString()},
		{equalID, cutoff, uuid.NewString()},
		{afterID, cutoff.Add(time.Second), uuid.NewString()},
	} {
		attachments, err := json.Marshal([]map[string]string{{"file_id": item.file, "type": "image"}})
		require.NoError(t, err)
		_, err = pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,attachments,mentions,created_at) VALUES($1,$2,'dm',$3,'payload',$4::jsonb,'[]'::jsonb,$5)`, item.id, chatID, senderID, attachments, item.created)
		require.NoError(t, err)
	}
	opID := uuid.New()
	requestHash := sha256.Sum256([]byte("immutable purge request"))
	work, err := store.StartManagedChatPurge(ctx, opID, chatID, cutoff, requestHash[:])
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{beforeID, equalID}, work.MessageIDs)
	require.Len(t, work.MessageAttachments, 2)
	require.Len(t, work.MessageAttachments[0].FileIDs, 1)

	_, err = pool.Exec(ctx, `UPDATE messages SET content='mutated',attachments='[]'::jsonb WHERE id=$1`, beforeID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `DELETE FROM messages WHERE id=$1`, equalID)
	require.NoError(t, err)
	replayed, err := store.StartManagedChatPurge(ctx, opID, chatID, cutoff, requestHash[:])
	require.NoError(t, err)
	require.Equal(t, work.OperationID, replayed.OperationID)
	require.Equal(t, work.ChatID, replayed.ChatID)
	require.True(t, work.PurgeAfter.Equal(replayed.PurgeAfter))
	require.Equal(t, work.RequestSHA256, replayed.RequestSHA256)
	require.Equal(t, work.State, replayed.State)
	require.Equal(t, work.MessageIDs, replayed.MessageIDs)
	require.Equal(t, work.MessageAttachments, replayed.MessageAttachments, "replay must use the original frozen payload and File set")
	fileReceipt, searchReceipt := sha256.Sum256([]byte("file receipt")), sha256.Sum256([]byte("search receipt"))
	completed, err := store.CompleteManagedChatPurge(ctx, opID, fileReceipt[:], searchReceipt[:])
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", completed.State)
	require.NotNil(t, completed.CompletedAt)
	require.Equal(t, fileReceipt[:], completed.FileReceiptSHA256)
	require.Equal(t, searchReceipt[:], completed.SearchReceiptSHA256)
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id=ANY($1)`, work.MessageIDs).Scan(&remaining))
	require.Zero(t, remaining, "all frozen payload rows must be physically deleted")
	completionReplay, err := store.CompleteManagedChatPurge(ctx, opID, fileReceipt[:], searchReceipt[:])
	require.NoError(t, err)
	require.Equal(t, completed.CompletedAt, completionReplay.CompletedAt)
	changedReceipt := sha256.Sum256([]byte("changed receipt"))
	_, err = store.CompleteManagedChatPurge(ctx, opID, changedReceipt[:], searchReceipt[:])
	require.ErrorIs(t, err, ErrManagedChatPurgeOperationConflict)

	changed := sha256.Sum256([]byte("changed purge request"))
	_, err = store.StartManagedChatPurge(ctx, opID, chatID, cutoff, changed[:])
	require.ErrorIs(t, err, ErrManagedChatPurgeOperationConflict)
}

func TestManagedChatPurgeCannotStartBeforeFrozenCutoff(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	requestHash := sha256.Sum256([]byte("future cutoff"))
	_, err := (&MessagesStore{Pool: pool}).StartManagedChatPurge(ctx, uuid.New(), uuid.New(), time.Now().UTC().Add(time.Hour), requestHash[:])
	require.ErrorIs(t, err, ErrManagedChatPurgeNotDue)
}
