package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/messaging/internal/messageid"
)

func TestScheduledAttachmentCancelAfterAcquireRollsBackAndReleases(t *testing.T) {
	s, message, request := attachmentIntentFixture(t)
	ctx := context.Background()
	at := time.Now().Add(time.Hour)
	id, err := messageid.NewMessageID()
	require.NoError(t, err)
	message.ID = id
	message.ClientMessageID = nil
	scheduled, err := s.CreateScheduled(ctx, ScheduledRecord{ScheduledMessageRow: ScheduledMessageRow{ID: uuid.New(), ChatID: message.ChatID, SenderProfileID: message.SenderProfileID, PayloadJSON: `{"content":"attachment"}`, ScheduleKind: ScheduledMessageScheduleAt, ScheduledAt: &at}, DispatchMessageID: id})
	require.NoError(t, err)
	event, err := messageevents.NewMessageSentOutbox(id.String(), message.ChatID.String(), message.SenderProfileID.String(), false, "", false, "document", false)
	require.NoError(t, err)
	var acquired *filev1.AcquireFileReferencesRequest
	_, err = s.InsertMessageWithReferencesAndOutbox(WithScheduledDispatch(ctx, *scheduled), message, nil, request, func(_ context.Context, bound *filev1.AcquireFileReferencesRequest) error {
		acquired = proto.Clone(bound).(*filev1.AcquireFileReferencesRequest)
		return s.CancelScheduled(ctx, scheduled.ID, scheduled.SenderProfileID)
	}, []messageevents.OutboxEvent{event})
	require.ErrorIs(t, err, ErrScheduledState)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM messages`).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, s.Pool.QueryRow(ctx, `SELECT count(*) FROM message_event_outbox`).Scan(&count))
	require.Zero(t, count)
	current, err := s.GetOwnedScheduled(ctx, scheduled.ID, scheduled.SenderProfileID)
	require.NoError(t, err)
	require.Equal(t, ScheduledMessageCancelled, current.Status)
	_, err = s.Pool.Exec(ctx, `UPDATE messaging_attachment_send_intents SET expires_at=clock_timestamp()-interval '1 second' WHERE message_id=$1`, id)
	require.NoError(t, err)
	releases := 0
	require.NoError(t, s.ReconcileAttachmentIntents(ctx, func(_ context.Context, bound *filev1.ReleaseFileReferencesRequest) error {
		releases++
		require.True(t, proto.Equal(acquired.References[0], bound.References[0]))
		return nil
	}))
	require.Equal(t, 1, releases)
}
