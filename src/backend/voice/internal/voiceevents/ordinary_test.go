package voiceevents

import (
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestOrdinaryDurablePublisherPreservesFiveCompatibilityEnvelopes(t *testing.T) {
	for _, kind := range []string{"incoming", "accepted", "declined", "missed", "ended"} {
		t.Run(kind, func(t *testing.T) {
			id := uuid.New()
			env := &eventsv1.VoiceStreamEvent{EventId: id.String(), OccurredAt: timestamppb.Now()}
			switch kind {
			case "incoming":
				env.Payload = &eventsv1.VoiceStreamEvent_CallIncoming{CallIncoming: &eventsv1.CallIncoming{RoomId: uuid.NewString()}}
			case "accepted":
				env.Payload = &eventsv1.VoiceStreamEvent_CallAccepted{CallAccepted: &eventsv1.CallAccepted{RoomId: uuid.NewString()}}
			case "declined":
				env.Payload = &eventsv1.VoiceStreamEvent_CallDeclined{CallDeclined: &eventsv1.CallDeclined{RoomId: uuid.NewString()}}
			case "missed":
				env.Payload = &eventsv1.VoiceStreamEvent_CallMissed{CallMissed: &eventsv1.CallMissed{RoomId: uuid.NewString()}}
			case "ended":
				env.Payload = &eventsv1.VoiceStreamEvent_CallEnded{CallEnded: &eventsv1.CallEnded{RoomId: uuid.NewString(), ProfileIds: []string{uuid.NewString()}}}
			}
			b, err := proto.Marshal(env)
			require.NoError(t, err)
			js := &recordingJetStream{}
			p := &JetStreamPublisher{js: js}
			subject := "voice.call_" + kind
			require.NoError(t, p.PublishOrdinaryEvent(t.Context(), id, subject, b))
			require.NoError(t, p.PublishOrdinaryEvent(t.Context(), id, subject, b))
			require.Len(t, js.messages, 2)
			for _, m := range js.messages {
				require.Equal(t, b, m.Data)
				require.Equal(t, subject, m.Subject)
				require.Equal(t, id.String(), m.Header.Get("Nats-Msg-Id"))
			}
			wrong := "voice.call_ended"
			if kind == "ended" {
				wrong = "voice.call_incoming"
			}
			require.Error(t, p.PublishOrdinaryEvent(t.Context(), id, wrong, b))
			require.Error(t, p.PublishOrdinaryEvent(t.Context(), uuid.New(), subject, b))
			require.Len(t, js.messages, 2)
			failing := &JetStreamPublisher{js: failingJetStream{}}
			require.Error(t, failing.PublishOrdinaryEvent(t.Context(), id, subject, b))
			noAck := &JetStreamPublisher{js: ordinaryMissingAck{}}
			require.Error(t, noAck.PublishOrdinaryEvent(t.Context(), id, subject, b))
		})
	}
}

type ordinaryMissingAck struct{}

func (ordinaryMissingAck) PublishMsg(*nats.Msg, ...nats.PubOpt) (*nats.PubAck, error) {
	return nil, nil
}
