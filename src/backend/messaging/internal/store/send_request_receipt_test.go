package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/messaging/internal/messageevents"
)

func TestExactSendReceiptAtomicallyResolvesConcurrentChangedRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL atomic receipt/outbox")
	}
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	st := &MessagesStore{Pool: pool}
	chat, sender, client := uuid.New(), uuid.New(), uuid.New()
	rows := []MessageRow{{ID: uuid.New(), ChatID: chat, SenderProfileID: sender, ClientMessageID: &client, ChatType: "group", Type: "regular", Content: "first", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]", AdmissionRequestHash: "sha256:" + strings.Repeat("a", 64), AdmissionRequestRPC: "/voice.messaging.v1.MessagingService/SendMessage"}}
	second := rows[0]
	second.ID = uuid.New()
	second.Content = "other"
	second.AdmissionRequestHash = "sha256:" + strings.Repeat("b", 64)
	rows = append(rows, second)
	type result struct {
		row MessageRow
		err error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	for _, row := range rows {
		event, err := messageevents.NewMessageSentOutbox(row.ID.String(), chat.String(), sender.String(), false, "", false, "text", false)
		require.NoError(t, err)
		go func() {
			<-start
			_, _, err := st.InsertMessageWithOutbox(ctx, row, nil, []messageevents.OutboxEvent{event})
			results <- result{row, err}
		}()
	}
	close(start)
	successes, conflicts := 0, 0
	var winner MessageRow
	for range rows {
		result := <-results
		if result.err == nil {
			successes++
			winner = result.row
		} else if errors.Is(result.err, ErrSendReceiptConflict) {
			conflicts++
		} else {
			require.NoError(t, result.err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	var messages, receipts, events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_send_receipts),(SELECT count(*) FROM message_event_outbox)`).Scan(&messages, &receipts, &events))
	require.Equal(t, []int{1, 1, 1}, []int{messages, receipts, events})
	saved, err := st.GetExactSendReceipt(ctx, chat, sender, client, winner.AdmissionRequestRPC, winner.AdmissionRequestHash)
	require.NoError(t, err)
	require.Equal(t, winner.ID, saved.ID)
	_, err = st.GetExactSendReceipt(ctx, chat, sender, client, "/voice.messaging.v1.MessagingService/SendGameEventMessage", winner.AdmissionRequestHash)
	require.ErrorIs(t, err, ErrSendReceiptConflict, "a digest is not authority for another RPC")
}
