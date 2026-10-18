package mmevents

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

type rosterPublication struct {
	nats.JetStreamContext
	messages []*nats.Msg
	err      error
	noAck    bool
}

func (p *rosterPublication) PublishMsg(msg *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	p.messages = append(p.messages, msg)
	if p.err != nil {
		return nil, p.err
	}
	if p.noAck {
		return nil, nil
	}
	return &nats.PubAck{Stream: streamName, Sequence: 1}, nil
}

func TestRosterCancellationPublishesFrozenBytesAndRequiresValidBindingAndAck(t *testing.T) {
	id := uuid.New()
	fact := &eventsv1.SearchCancelled{SessionId: uuid.NewString(), ProfileId: uuid.NewString(), Reason: "VOICE_ROSTER_CHANGED"}
	env := &eventsv1.MatchmakingStreamEvent{EventId: id.String(), OccurredAt: timestamppb.New(time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC)), Payload: &eventsv1.MatchmakingStreamEvent_SearchCancelled{SearchCancelled: fact}}
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(env)
	require.NoError(t, err)
	js := &rosterPublication{}
	publisher := &JetStreamPublisher{js: js}
	for range 2 {
		require.NoError(t, publisher.PublishRosterCancellation(context.Background(), id, raw))
	}
	require.Len(t, js.messages, 2)
	for _, message := range js.messages {
		require.Equal(t, subjectSearchCancel, message.Subject)
		require.Equal(t, id.String(), message.Header.Get(nats.MsgIdHdr))
		require.Equal(t, raw, message.Data)
	}
	require.Error(t, publisher.PublishRosterCancellation(context.Background(), uuid.New(), raw))
	fact.Reason = ""
	invalid, err := proto.Marshal(env)
	require.NoError(t, err)
	require.Error(t, publisher.PublishRosterCancellation(context.Background(), id, invalid))
	require.Len(t, js.messages, 2)
	js.noAck = true
	require.Error(t, publisher.PublishRosterCancellation(context.Background(), id, raw))
	js.noAck = false
	js.err = errors.New("PubAck unknown")
	require.Error(t, publisher.PublishRosterCancellation(context.Background(), id, raw))
}
