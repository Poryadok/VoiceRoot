package messageevents

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

func TestScheduledOutboxBindsMetadataWithoutReplacingEventIdentity(t *testing.T) {
	event, err := NewMessageSentOutbox(uuid.NewString(), uuid.NewString(), uuid.NewString(), false, "", false, "text", true)
	require.NoError(t, err)
	at := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	scheduled, err := WithScheduledMetadata(event, &at)
	require.NoError(t, err)
	require.Equal(t, event.EventID, scheduled.EventID)
	require.Equal(t, event.Headers, scheduled.Headers)
	var envelope eventsv1.MessageStreamEvent
	require.NoError(t, proto.Unmarshal(scheduled.Payload, &envelope))
	require.True(t, envelope.GetMessageSent().WasScheduled)
	require.True(t, envelope.GetMessageSent().SendSilent)
	require.Equal(t, at, envelope.GetMessageSent().ScheduledAt.AsTime())
	online, err := WithScheduledMetadata(event, nil)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(online.Payload, &envelope))
	require.True(t, envelope.GetMessageSent().WasScheduled)
	require.Nil(t, envelope.GetMessageSent().ScheduledAt)
}
