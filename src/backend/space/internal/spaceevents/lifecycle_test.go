package spaceevents

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/spacecore"
)

func TestLifecycleEventsPublishCanonicalEnvelopeAndDeduplicateLostResponseRetry(t *testing.T) {
	server := startJSTestServer(t)
	bootstrapStream(t, server.ClientURL())
	publisher, err := NewJetStreamPublisher(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = publisher.Close() })
	now := time.Now().UTC().Truncate(time.Microsecond)
	for _, kind := range []string{"space.deletion_scheduled", "space.restored", "space.deleted"} {
		record := spacecore.LifecycleOutboxRecord{EventID: uuid.NewString(), SpaceID: uuid.NewString(), DeletionOperationID: uuid.NewString(), Generation: 2, EventType: kind, OccurredAt: now}
		require.NoError(t, publisher.PublishLifecycle(context.Background(), record, now.Add(-time.Hour)))
		require.NoError(t, publisher.PublishLifecycle(context.Background(), record, now.Add(-time.Hour)), "lost local acknowledgement retries the same event ID and bytes")
		info, err := publisher.js.StreamInfo(streamName)
		require.NoError(t, err)
		message, err := publisher.js.(nats.JetStreamContext).GetMsg(streamName, info.State.LastSeq)
		require.NoError(t, err)
		require.Equal(t, record.EventID, message.Header.Get(nats.MsgIdHdr))
		require.Equal(t, kind, message.Subject)
		envelope := &eventsv1.ChatStreamEvent{}
		require.NoError(t, proto.Unmarshal(message.Data, envelope))
		require.Equal(t, record.EventID, envelope.EventId)
		require.Equal(t, now, envelope.OccurredAt.AsTime())
		switch kind {
		case "space.deletion_scheduled":
			require.Equal(t, now.Add(7*24*time.Hour), envelope.GetSpaceDeletionScheduled().PurgeAfter.AsTime())
		case "space.restored":
			require.Equal(t, record.DeletionOperationID, envelope.GetSpaceRestored().DeletionOperationId)
		case "space.deleted":
			require.Equal(t, now.Add(-time.Hour), envelope.GetSpaceDeleted().PurgeDecidedAt.AsTime())
		}
	}
	info, err := publisher.js.StreamInfo(streamName)
	require.NoError(t, err)
	require.Equal(t, uint64(3), info.State.Msgs)
}

func TestLifecyclePublishWithholdsCompletionOnInvalidAckAndTransportFailure(t *testing.T) {
	record := spacecore.LifecycleOutboxRecord{EventID: uuid.NewString(), SpaceID: uuid.NewString(), DeletionOperationID: uuid.NewString(), Generation: 1, EventType: "space.restored", OccurredAt: time.Now().UTC()}
	for _, ack := range []*nats.PubAck{nil, {Stream: "wrong", Sequence: 1}, {Stream: streamName}} {
		capture := &preparedMessageJetStream{ack: ack}
		publisher := &JetStreamPublisher{js: capture}
		publisher.ensureOnce.Do(func() {})
		require.Error(t, publisher.PublishLifecycle(context.Background(), record, time.Time{}))
	}
	failure := errors.New("lost server response")
	capture := &preparedMessageJetStream{err: failure}
	publisher := &JetStreamPublisher{js: capture}
	publisher.ensureOnce.Do(func() {})
	require.ErrorIs(t, publisher.PublishLifecycle(context.Background(), record, time.Time{}), failure)
}
