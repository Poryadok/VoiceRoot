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
	"voice/backend/role/internal/store"
)

type voicePolicySourceFake []store.VoicePolicyInvalidation

func (s voicePolicySourceFake) ReadLatestVoicePolicyInvalidations(_ context.Context, after uuid.UUID, limit int) ([]store.VoicePolicyInvalidation, error) {
	rows := make([]store.VoicePolicyInvalidation, 0, limit)
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

type voicePolicyPublisherFake struct {
	events []*eventsv1.RoleStreamEvent
	ack    *nats.PubAck
	err    error
}

func (p *voicePolicyPublisherFake) PublishVoiceRoomPolicyInvalidated(_ context.Context, event *eventsv1.RoleStreamEvent) (*nats.PubAck, error) {
	p.events = append(p.events, event)
	return p.ack, p.err
}

func TestVoicePolicyDispatcherAdvancesOnlyAfterPubAckAndReplaysStableSnapshot(t *testing.T) {
	spaceID, eventID := uuid.New(), uuid.New()
	row := store.VoicePolicyInvalidation{EventID: eventID, SpaceID: spaceID, Epoch: 9, CreatedAt: time.Unix(1, 0).UTC()}
	publisher := &voicePolicyPublisherFake{err: errors.New("publish unavailable")}
	dispatcher := NewVoicePolicyDispatcher(voicePolicySourceFake{row}, publisher)
	require.Error(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, uint64(0), dispatcher.acked[spaceID])

	publisher.err = nil
	publisher.ack = &nats.PubAck{Stream: "role_events", Sequence: 1}
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	require.Equal(t, uint64(9), dispatcher.acked[spaceID])
	require.NoError(t, dispatcher.DispatchOnce(context.Background()))
	require.Len(t, publisher.events, 2, "same immutable row is skipped after confirmed source progress")
	require.Equal(t, eventID.String(), publisher.events[0].GetEventId())
	require.Equal(t, eventID.String(), publisher.events[1].GetEventId())
}
