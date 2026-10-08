package store

import (
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"testing"
	"time"

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
		switch table {
		case "read_receipts", "read_positions":
			query += "chat_id=$1 AND profile_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, chatID, viewer).Scan(&rows), table)
		case "message_hides":
			query += "message_id=$1 AND profile_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, messageID, viewer).Scan(&rows), table)
		case "reactions":
			query += "message_id=$1 AND profile_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, messageID, viewer).Scan(&rows), table)
		case "pins":
			query += "chat_id=$1 AND message_id=$2"
			require.NoError(t, pool.QueryRow(ctx, query, chatID, messageID).Scan(&rows), table)
		default:
			t.Fatalf("unexpected side table %q", table)
		}
		require.Zero(t, rows, table+" must remain empty after stale mutation attempts")
	}
}

func TestClearPublicReadReceiptsCannotChangeFrozenPurgeEventSet(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}

	freeChat, frozenChat := uuid.New(), uuid.New()
	profileID, peerID, senderID := uuid.New(), uuid.New(), uuid.New()
	freeMessage, frozenMessage := uuid.New(), uuid.New()
	for _, row := range []struct{ chatID, messageID uuid.UUID }{{freeChat, freeMessage}, {frozenChat, frozenMessage}} {
		_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,attachments,mentions,created_at)
VALUES($1,$2,'dm',$3,'payload','[]'::jsonb,'[]'::jsonb,clock_timestamp()-interval '1 day')`, row.messageID, row.chatID, senderID)
		require.NoError(t, err)
		require.NoError(t, store.UpsertReadReceipt(ctx, row.chatID, profileID, row.messageID))
	}
	// Peer cursors also produce revocation events, although only the opted-out
	// profile's own cursor is cleared.
	require.NoError(t, store.UpsertReadReceipt(ctx, frozenChat, peerID, frozenMessage))

	// This immutable event is the exact set frozen by StartManagedChatPurge.
	eventID := uuid.New()
	payload := []byte("original frozen message event")
	digest := sha256.Sum256(payload)
	_, err := pool.Exec(ctx, `INSERT INTO message_event_outbox(event_id,subject,message_id,chat_id,payload_bytes,payload_sha256)
VALUES($1,'message.sent',$2,$3,$4,$5)`, eventID, frozenMessage, frozenChat, payload, digest[:])
	require.NoError(t, err)
	requestHash := sha256.Sum256([]byte("frozen receipt-clear request"))
	operationID := uuid.New()
	work, err := store.StartManagedChatPurge(ctx, operationID, frozenChat, time.Now().UTC().Add(-time.Hour), requestHash[:])
	require.NoError(t, err)
	require.EqualValues(t, 1, work.EventCount)

	_, err = store.ClearPublicReadReceiptsWithOutbox(ctx, profileID, map[uuid.UUID]uuid.UUID{freeChat: peerID, frozenChat: peerID})
	require.ErrorIs(t, err, ErrMessageMutationPurging)

	for _, row := range []struct{ chatID, messageID uuid.UUID }{{freeChat, freeMessage}, {frozenChat, frozenMessage}} {
		public, _, err := store.GetReadReceipt(ctx, row.chatID, profileID)
		require.NoError(t, err)
		require.Equal(t, row.messageID, *public, "a multi-chat rejection must leave every receipt unchanged")
	}
	var eventCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox`).Scan(&eventCount))
	require.Equal(t, 1, eventCount, "failed clear must not enqueue a revocation outside the frozen event set")
	var savedCount int64
	var savedHash []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_count,event_set_sha256 FROM managed_chat_purge_operations WHERE operation_id=$1`, operationID).Scan(&savedCount, &savedHash))
	require.Equal(t, work.EventCount, savedCount)
	require.Equal(t, work.EventSetSHA256, savedHash)
	require.ErrorIs(t, store.RequireManagedChatPurgeEventsPublished(ctx, operationID), ErrMessageEventOutboxUnavailable,
		"the original event remains an explicit barrier until its positive PubAck")

	_, err = pool.Exec(ctx, `UPDATE message_event_outbox SET payload_bytes=NULL,headers='{}'::jsonb,pubacked_at=clock_timestamp(),puback_sequence=11,payload_pruned_at=clock_timestamp() WHERE event_id=$1`, eventID)
	require.NoError(t, err)
	require.NoError(t, store.RequireManagedChatPurgeEventsPublished(ctx, operationID), "the original event's PubAck still gates the unchanged frozen set")
	fileReceipt, searchReceipt := sha256.Sum256([]byte("file receipt")), sha256.Sum256([]byte("search receipt"))
	_, err = store.CompleteManagedChatPurge(ctx, operationID, fileReceipt[:], searchReceipt[:])
	require.NoError(t, err)

	// The purge removed the frozen chat's receipt. The unrelated chat can now
	// complete the previously all-or-nothing opt-out without partial state.
	revoked, err := store.ClearPublicReadReceiptsWithOutbox(ctx, profileID, map[uuid.UUID]uuid.UUID{freeChat: peerID, frozenChat: peerID})
	require.NoError(t, err)
	require.Equal(t, []PublicReadReceipt{{ChatID: freeChat, ProfileID: profileID, MessageID: freeMessage, RecipientProfileID: peerID}}, revoked)
	public, _, err := store.GetReadReceipt(ctx, freeChat, profileID)
	require.NoError(t, err)
	require.Nil(t, public)
}

func TestClearPublicReadReceiptRevocationIsIncludedInPurgeEventSet(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	store := &MessagesStore{Pool: pool}
	chatID, profileID, peerID, senderID, messageID := uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO messages(id,chat_id,chat_type,sender_profile_id,content,attachments,mentions,created_at)
VALUES($1,$2,'dm',$3,'payload','[]'::jsonb,'[]'::jsonb,clock_timestamp()-interval '1 day')`, messageID, chatID, senderID)
	require.NoError(t, err)
	require.NoError(t, store.UpsertReadReceipt(ctx, chatID, profileID, messageID))
	revoked, err := store.ClearPublicReadReceiptsWithOutbox(ctx, profileID, map[uuid.UUID]uuid.UUID{chatID: peerID})
	require.NoError(t, err)
	require.Equal(t, []PublicReadReceipt{{ChatID: chatID, ProfileID: profileID, MessageID: messageID, RecipientProfileID: peerID}}, revoked)

	requestHash := sha256.Sum256([]byte("pre-freeze revocation"))
	operationID := uuid.New()
	work, err := store.StartManagedChatPurge(ctx, operationID, chatID, time.Now().UTC().Add(-time.Hour), requestHash[:])
	require.NoError(t, err)
	require.EqualValues(t, 1, work.EventCount)
	var subject string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outbox.subject
FROM managed_chat_purge_events AS purge_event
JOIN message_event_outbox AS outbox USING (event_id)
WHERE purge_event.operation_id=$1`, operationID).Scan(&subject))
	require.Equal(t, "message.read_receipt_revoked", subject)
	require.ErrorIs(t, store.RequireManagedChatPurgeEventsPublished(ctx, operationID), ErrMessageEventOutboxUnavailable)

	var eventID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id FROM managed_chat_purge_events WHERE operation_id=$1`, operationID).Scan(&eventID))
	_, err = pool.Exec(ctx, `UPDATE message_event_outbox SET payload_bytes=NULL,headers='{}'::jsonb,pubacked_at=clock_timestamp(),puback_sequence=12,payload_pruned_at=clock_timestamp() WHERE event_id=$1`, eventID)
	require.NoError(t, err)
	require.NoError(t, store.RequireManagedChatPurgeEventsPublished(ctx, operationID))
	fileReceipt, searchReceipt := sha256.Sum256([]byte("file receipt")), sha256.Sum256([]byte("search receipt"))
	_, err = store.CompleteManagedChatPurge(ctx, operationID, fileReceipt[:], searchReceipt[:])
	require.NoError(t, err)
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
