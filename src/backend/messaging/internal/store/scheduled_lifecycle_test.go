package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"sync"
	"testing"
	"time"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/messageid"
)

func TestScheduledLifecycleOwnerRevisionAndOriginalRetry(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	at := time.Now().UTC().Add(time.Hour)
	key := uuid.New()
	input := ScheduledRecord{ScheduledMessageRow: ScheduledMessageRow{ID: uuid.New(), ChatID: uuid.New(), SenderProfileID: uuid.New(), ClientMessageID: &key, PayloadJSON: `{"content":"first"}`, ScheduleKind: ScheduledMessageScheduleAt, ScheduledAt: &at}, RequestRPC: "SendMessage", RequestHash: "original", DispatchMessageID: uuid.New()}
	first, err := s.CreateScheduled(ctx, input)
	require.NoError(t, err)
	_, err = s.GetOwnedScheduled(ctx, first.ID, uuid.New())
	require.ErrorIs(t, err, pgx.ErrNoRows)
	rows, err := s.ListPendingScheduled(ctx, first.ChatID, uuid.New(), nil, 10)
	require.NoError(t, err)
	require.Empty(t, rows)
	changed := *first
	changed.PayloadJSON = `{"content":"edited"}`
	updated, err := s.UpdateScheduled(ctx, changed)
	require.NoError(t, err)
	require.Greater(t, updated.Revision, first.Revision)
	_, err = s.UpdateScheduled(ctx, changed)
	require.ErrorIs(t, err, ErrScheduledState)
	retry, err := s.CreateScheduled(ctx, input)
	require.NoError(t, err)
	require.Equal(t, updated.PayloadJSON, retry.PayloadJSON)
	require.Equal(t, first.ID, retry.ID)
	input.RequestHash = "different"
	_, err = s.CreateScheduled(ctx, input)
	require.ErrorIs(t, err, ErrSendReceiptConflict)
	require.NoError(t, s.CancelScheduled(ctx, first.ID, first.SenderProfileID))
	require.NoError(t, s.CancelScheduled(ctx, first.ID, first.SenderProfileID))
	rows, err = s.ListPendingScheduled(ctx, first.ChatID, first.SenderProfileID, nil, 10)
	require.NoError(t, err)
	require.Empty(t, rows)
	_, err = s.UpdateScheduled(ctx, *updated)
	require.ErrorIs(t, err, ErrScheduledState)
	var events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox`).Scan(&events))
	require.Zero(t, events)
}

func TestScheduledDispatchAtomicRaceAndSharedNamespace(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	chat, owner := uuid.New(), uuid.New()
	at := time.Now().Add(time.Hour)
	create := func(key *uuid.UUID) *ScheduledRecord {
		t.Helper()
		dispatch, err := messageid.NewMessageID()
		require.NoError(t, err)
		row, err := s.CreateScheduled(ctx, ScheduledRecord{ScheduledMessageRow: ScheduledMessageRow{ID: uuid.New(), ChatID: chat, SenderProfileID: owner, ClientMessageID: key, PayloadJSON: `{"content":"scheduled"}`, ScheduleKind: ScheduledMessageScheduleAt, ScheduledAt: &at}, RequestRPC: "SendMessage", RequestHash: "original", DispatchMessageID: dispatch})
		require.NoError(t, err)
		return row
	}
	dispatch := func(row *ScheduledRecord) error {
		event, err := messageevents.NewMessageSentOutbox(row.DispatchMessageID.String(), chat.String(), owner.String(), false, "", false, "text", false)
		if err != nil {
			return err
		}
		_, _, err = s.InsertMessageWithOutbox(WithScheduledDispatch(ctx, *row), MessageRow{ID: row.DispatchMessageID, ChatID: chat, ChatType: "dm", SenderProfileID: owner, Content: "scheduled", Type: "regular", ContentType: "text", AttachmentsJSON: "[]", MentionsJSON: "[]"}, nil, []messageevents.OutboxEvent{event})
		return err
	}
	stale := create(nil)
	changed := *stale
	changed.PayloadJSON = `{"content":"new"}`
	_, err := s.UpdateScheduled(ctx, changed)
	require.NoError(t, err)
	require.ErrorIs(t, dispatch(stale), ErrScheduledState)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox`).Scan(&count))
	require.Zero(t, count)
	for i := 0; i < 8; i++ {
		row := create(nil)
		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); <-start; errs <- dispatch(row) }()
		go func() { defer wg.Done(); <-start; errs <- s.CancelScheduled(ctx, row.ID, owner) }()
		close(start)
		wg.Wait()
		close(errs)
		wins := 0
		for err := range errs {
			if err == nil {
				wins++
			} else {
				require.ErrorIs(t, err, ErrScheduledState)
			}
		}
		require.Equal(t, 1, wins)
		saved, err := s.GetOwnedScheduled(ctx, row.ID, owner)
		require.NoError(t, err)
		var messages, events int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messages WHERE id=$1`, row.DispatchMessageID).Scan(&messages))
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox WHERE message_id=$1`, row.DispatchMessageID).Scan(&events))
		if saved.Status == ScheduledMessageSent {
			require.Equal(t, 1, messages)
			require.Equal(t, 1, events)
			require.NotNil(t, saved.DispatchEventID)
		} else {
			require.Equal(t, ScheduledMessageCancelled, saved.Status)
			require.Zero(t, messages)
			require.Zero(t, events)
		}
	}
	key := uuid.New()
	scheduled := create(&key)
	id, err := messageid.NewMessageID()
	require.NoError(t, err)
	_, _, err = s.InsertMessageWithOutbox(ctx, MessageRow{ID: id, ChatID: chat, SenderProfileID: owner, Content: "immediate", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", ClientMessageID: &key}, nil, nil)
	require.True(t, IsSendNamespaceConflict(err), "immediate insert cannot bypass scheduled namespace")
	// Physical purge releases this private namespace rather than keeping raw IDs.
	_, err = pool.Exec(ctx, `DELETE FROM scheduled_messages WHERE id=$1`, scheduled.ID)
	require.NoError(t, err)
	_, _, err = s.InsertMessageWithOutbox(ctx, MessageRow{ID: id, ChatID: chat, SenderProfileID: owner, Content: "immediate", Type: "regular", AttachmentsJSON: "[]", MentionsJSON: "[]", ClientMessageID: &key}, nil, nil)
	require.NoError(t, err)
	_, err = s.CreateScheduled(ctx, *scheduled)
	require.ErrorIs(t, err, ErrSendReceiptConflict, "schedule cannot bypass immediate namespace")
}
