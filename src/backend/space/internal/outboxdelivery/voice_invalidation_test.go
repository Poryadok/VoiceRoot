package outboxdelivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/store"
)

type voiceInvalidationSourceFake []store.VoiceAccessInvalidation

func (s voiceInvalidationSourceFake) ReadLatestVoiceAccessInvalidations(_ context.Context, after uuid.UUID, limit int) ([]store.VoiceAccessInvalidation, error) {
	rows := make([]store.VoiceAccessInvalidation, 0, limit)
	for _, row := range s {
		if row.SpaceID.String() > after.String() {
			rows = append(rows, row)
			if len(rows) == limit {
				break
			}
		}
	}
	return rows, nil
}

type voiceInvalidationPublisherFake struct {
	events []*eventsv1.ChatStreamEvent
	ack    *nats.PubAck
	err    error
}

func (p *voiceInvalidationPublisherFake) PublishVoiceRoomAccessInvalidated(_ context.Context, event *eventsv1.ChatStreamEvent) (*nats.PubAck, error) {
	p.events = append(p.events, event)
	return p.ack, p.err
}

func TestVoiceInvalidationDispatcherAdvancesOnlyAfterPubAckAndReplaysStableSnapshot(t *testing.T) {
	spaceID, eventID := uuid.New(), uuid.New()
	row := store.VoiceAccessInvalidation{EventID: eventID, SpaceID: spaceID, Epoch: 7, CreatedAt: time.Unix(1, 0).UTC()}
	publisher := &voiceInvalidationPublisherFake{err: errors.New("publish unavailable")}
	dispatcher := NewVoiceInvalidationDispatcher(voiceInvalidationSourceFake{row}, publisher)
	require.Error(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, uint64(0), dispatcher.acked[spaceID])

	publisher.err = nil
	publisher.ack = &nats.PubAck{Stream: "chat_events", Sequence: 1}
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, uint64(7), dispatcher.acked[spaceID])
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	require.Len(t, publisher.events, 2, "same immutable row is skipped after confirmed source progress")
	require.Equal(t, eventID.String(), publisher.events[0].GetEventId())
	require.Equal(t, eventID.String(), publisher.events[1].GetEventId())
}
