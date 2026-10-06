package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"voice/backend/messaging/internal/messageevents"
)

func TestEnqueueMessageEventNormalizesNilHeadersToObject(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)

	event := messageevents.OutboxEvent{
		EventID: uuid.New(), MessageID: uuid.New(), ChatID: uuid.New(),
		Subject: "message.sent", Payload: []byte("immutable-event-bytes"), Headers: nil,
	}
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, enqueueMessageEvent(ctx, tx, event))
	require.NoError(t, tx.Commit(ctx))

	var headers string
	require.NoError(t, pool.QueryRow(ctx, `SELECT headers::text FROM message_event_outbox WHERE event_id=$1`, event.EventID).Scan(&headers))
	require.Equal(t, "{}", headers, "the schema and dispatcher require a JSON object, including for empty headers")
}

func TestPurgedOrDeletedMessageCannotRecreateSideTableState(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)

	messages := &MessagesStore{Pool: pool}
	chatID, messageID, sender, viewer := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := messages.InsertMessage(ctx, MessageRow{
		ID: messageID, ChatID: chatID, SenderProfileID: sender,
		Content: "payload", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]",
	})
	require.NoError(t, err)
	deleteEvent := messageevents.OutboxEvent{EventID: uuid.New(), MessageID: messageID, ChatID: chatID, Subject: "message.deleted", Payload: []byte("delete")}
	require.NoError(t, messages.SoftDeleteMessageWithOutbox(ctx, chatID, uuid.Nil, messageID, sender, deleteEvent))

	require.NoError(t, messages.UpsertDeliveredCursorWithMutationFence(ctx, chatID, uuid.Nil, viewer, messageID), "late delivery ack is a stale no-op")
	require.ErrorIs(t, messages.HideMessageForProfileWithMutationFence(ctx, chatID, uuid.Nil, messageID, viewer), pgx.ErrNoRows)
	require.ErrorIs(t, messages.UpsertReadStateAndOutbox(ctx, chatID, uuid.Nil, viewer, messageID, false, nil), pgx.ErrNoRows)
	reactionEvent := messageevents.OutboxEvent{EventID: uuid.New(), MessageID: messageID, ChatID: chatID, Subject: "message.reaction_added", Payload: []byte("reaction")}
	require.ErrorIs(t, (&ReactionsStore{Pool: pool}).MutateReactionWithOutbox(ctx, chatID, uuid.Nil, messageID, viewer, ":)", true, reactionEvent), pgx.ErrNoRows)
	pinEvent := messageevents.OutboxEvent{EventID: uuid.New(), MessageID: messageID, ChatID: chatID, Subject: "message.pinned", Payload: []byte("pin")}
	require.ErrorIs(t, (&PinsStore{Pool: pool}).MutatePinWithOutbox(ctx, chatID, uuid.Nil, messageID, viewer, true, pinEvent), pgx.ErrNoRows)

	for _, table := range []string{"read_receipts", "read_positions", "message_hides", "reactions", "pins"} {
		var rows int
		query := "SELECT COUNT(*) FROM " + table + " WHERE "
		if table == "read_receipts" || table == "read_positions" {
			query += "chat_id=$1 AND profile_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, chatID, viewer).Scan(&rows), table)
		} else if table == "message_hides" {
			query += "message_id=$1 AND profile_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, messageID, viewer).Scan(&rows), table)
		} else if table == "reactions" {
			query += "message_id=$1 AND profile_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, messageID, viewer).Scan(&rows), table)
		} else {
			query += "chat_id=$1 AND message_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, chatID, messageID).Scan(&rows), table)
		}
		require.Zero(t, rows, table+" must remain empty after stale mutation attempts")
	}
}

func TestMessageEventOutboxDownRefusesAndPreservesUndeliveredPayload(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)

	eventID := uuid.New()
	payload := []byte("immutable-message-event-wire-bytes")
	digest := sha256.Sum256(payload)
	_, err := pool.Exec(ctx, `INSERT INTO message_event_outbox(event_id,subject,message_id,chat_id,payload_bytes,payload_sha256,headers)
VALUES($1,'message.sent',$2,$3,$4,$5,'{}'::jsonb)`, eventID, uuid.New(), uuid.New(), payload, digest[:])
	require.NoError(t, err)

	downPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "messaging_db", "000027_message_event_outbox.down.sql")
	downSQL, err := os.ReadFile(downPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(downSQL))
	require.Error(t, err, "downgrade must refuse to discard queued event bytes")

	var saved []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload_bytes FROM message_event_outbox WHERE event_id=$1`, eventID).Scan(&saved))
	require.Equal(t, payload, saved, "failed downgrade must preserve exact queued bytes")
}
