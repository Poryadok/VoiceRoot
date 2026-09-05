package mmevents

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

type captureJetStream struct {
	nats.JetStreamContext
	msg *nats.Msg
}

func (c *captureJetStream) PublishMsg(msg *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	c.msg = msg
	return &nats.PubAck{Stream: streamName, Sequence: 1}, nil
}

func TestJetStreamPublisher_SearchCancelledPublishesProtoEnvelope(t *testing.T) {
	const (
		sessionID = "session-cancelled"
		profileID = "profile-cancelled"
	)

	capture := &captureJetStream{}
	publisher := &JetStreamPublisher{js: capture}
	publisher.ensureOnce.Do(func() {})

	require.NoError(t, publisher.PublishSearchCancelled(context.Background(), sessionID, profileID))
	require.NotNil(t, capture.msg)
	require.Equal(t, subjectSearchCancel, capture.msg.Subject)

	var envelope eventsv1.MatchmakingStreamEvent
	require.NoError(t, proto.Unmarshal(capture.msg.Data, &envelope))
	require.NotEmpty(t, envelope.GetEventId())
	require.NotNil(t, envelope.GetOccurredAt())
	require.NoError(t, envelope.GetOccurredAt().CheckValid())
	require.False(t, envelope.GetOccurredAt().AsTime().IsZero())
	require.Equal(t, envelope.GetEventId(), capture.msg.Header.Get(nats.MsgIdHdr))

	cancelled := envelope.GetSearchCancelled()
	require.NotNil(t, cancelled)
	require.Equal(t, sessionID, cancelled.GetSessionId())
	require.Equal(t, profileID, cancelled.GetProfileId())
}
