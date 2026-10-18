package outboxdelivery

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/store"
)

type admissionEventSource struct {
	event    store.MemberAdmissionEvent
	marks    int
	failMark bool
}

func (s *admissionEventSource) ListPendingMemberAdmissionEvents(context.Context, int) ([]store.MemberAdmissionEvent, error) {
	return []store.MemberAdmissionEvent{s.event}, nil
}
func (s *admissionEventSource) MarkMemberAdmissionEventDelivered(_ context.Context, event store.MemberAdmissionEvent, stream string, sequence uint64) error {
	if event != s.event || stream != "chat_events" || sequence != 9 {
		return errors.New("different event ack")
	}
	s.marks++
	if s.failMark {
		return errors.New("DB unavailable after PubAck")
	}
	return nil
}

type admissionEventTransport struct {
	messages []*nats.Msg
	ack      *nats.PubAck
}

func (s *admissionEventTransport) Publish(_ context.Context, msg *nats.Msg) (*nats.PubAck, error) {
	s.messages = append(s.messages, msg)
	return s.ack, nil
}

func TestMemberAdmissionDispatcherRequiresAckAndReplaysIdenticalEventAfterMarkFailure(t *testing.T) {
	ctx := context.Background()
	source := &admissionEventSource{event: store.MemberAdmissionEvent{EventID: uuid.New(), OperationID: uuid.New(), SpaceID: uuid.New(), ProfileID: uuid.New(), Generation: 3, OccurredAt: time.Now().UTC()}}
	transport := &admissionEventTransport{}
	d := &MemberAdmissionDispatcher{Store: source, Publisher: transport}
	for _, ack := range []*nats.PubAck{nil, {Stream: "other", Sequence: 9}, {Stream: "chat_events"}} {
		transport.ack = ack
		require.Error(t, d.DispatchOnce(ctx))
		require.Zero(t, source.marks)
	}
	transport.ack = &nats.PubAck{Stream: "chat_events", Sequence: 9}
	source.failMark = true
	require.Error(t, d.DispatchOnce(ctx), "a PubAck with failed durable mark must retry")
	source.failMark = false
	require.NoError(t, d.DispatchOnce(ctx))
	require.Equal(t, 2, source.marks)
	first := transport.messages[0]
	for _, msg := range transport.messages {
		require.Equal(t, "space.member_joined", msg.Subject)
		require.Equal(t, source.event.EventID.String(), msg.Header.Get(nats.MsgIdHdr))
		require.Equal(t, first.Data, msg.Data, "restart/retry cannot mint a new event or wall clock")
	}
	envelope := &eventsv1.ChatStreamEvent{}
	require.NoError(t, proto.Unmarshal(first.Data, envelope))
	require.Equal(t, source.event.EventID.String(), envelope.EventId)
	require.Equal(t, source.event.OccurredAt, envelope.OccurredAt.AsTime())
	require.Equal(t, source.event.ProfileID.String(), envelope.GetSpaceMemberJoined().ProfileId)
}
