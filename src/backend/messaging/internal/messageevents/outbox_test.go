package messageevents

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

func TestEncodeOutboxEventPersistsExactStableEnvelopeAndHeaders(t *testing.T) {
	event := &eventsv1.MessageStreamEvent{
		EventId:    "11111111-1111-4111-8111-111111111111",
		OccurredAt: timestamppb.New(time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC)),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{MessageSent: &eventsv1.MessageSent{
			MessageId: "22222222-2222-4222-8222-222222222222",
			ChatId:    "33333333-3333-4333-8333-333333333333",
		}},
	}
	headers := map[string]string{"X-Voice-Thread-Parent-Id": "44444444-4444-4444-8444-444444444444"}

	first, err := encodeOutboxEvent(subjectMessageSent, event, headers)
	require.NoError(t, err)
	second, err := encodeOutboxEvent(subjectMessageSent, event, headers)
	require.NoError(t, err)
	require.Equal(t, "11111111-1111-4111-8111-111111111111", first.EventID)
	require.Equal(t, subjectMessageSent, first.Subject)
	require.Equal(t, headers, first.Headers)
	require.True(t, bytes.Equal(first.Payload, second.Payload), "persisted retry bytes must be deterministic")
	require.Equal(t, event.GetEventId(), first.EventID)
}
