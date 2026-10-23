package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	filev1 "voice.app/voice/file/v1"
)

func TestBotSendAdmissionRecheckedBeforeTextAndAttachmentWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL mutation transaction")
	}
	for _, attachments := range []bool{false, true} {
		t.Run(map[bool]string{false: "text", true: "attachments"}[attachments], func(t *testing.T) {
			st, row, request := attachmentIntentFixture(t)
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			denied := errors.New("current Bot installation revoked")
			checked := false
			acquired := false
			row.BotSendAdmission = func(attempt context.Context) (context.Context, context.CancelFunc, error) {
				checked = true
				require.Equal(t, ctx, attempt)
				if attachments {
					require.True(t, acquired)
				}
				return nil, nil, denied
			}
			var err error
			if attachments {
				_, err = st.InsertMessageWithReferences(ctx, row, nil, request, func(context.Context, *filev1.AcquireFileReferencesRequest) error { acquired = true; return nil })
			} else {
				row.AttachmentsJSON = "[]"
				_, _, err = st.InsertMessageWithOutbox(ctx, row, nil, nil)
			}
			require.ErrorIs(t, err, denied)
			require.True(t, checked)
			var messages, events int
			require.NoError(t, st.Pool.QueryRow(t.Context(), `SELECT (SELECT count(*) FROM messages),(SELECT count(*) FROM message_event_outbox)`).Scan(&messages, &events))
			require.Zero(t, messages)
			require.Zero(t, events)
		})
	}
}
func TestBotSendAdmissionShortenedDeadlineStopsInsert(t *testing.T) {
	if testing.Short() {
		t.Skip("requires PostgreSQL mutation transaction")
	}
	st, row, _ := attachmentIntentFixture(t)
	row.ID = uuid.New()
	row.AttachmentsJSON = "[]"
	row.BotSendAdmission = func(ctx context.Context) (context.Context, context.CancelFunc, error) {
		attempt, cancel := context.WithDeadline(ctx, time.Now().Add(-time.Second))
		return attempt, cancel, nil
	}
	_, _, err := st.InsertMessageWithOutbox(t.Context(), row, nil, nil)
	require.Error(t, err)
	var count int
	require.NoError(t, st.Pool.QueryRow(t.Context(), `SELECT count(*) FROM messages`).Scan(&count))
	require.Zero(t, count)
}
