package main

import (
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestVoiceParticipantFactsAckWithoutPublicFanout(t *testing.T) {
	for _, left := range []bool{false, true} {
		envelope := &eventsv1.VoiceStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now()}
		if left {
			envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantLeft{VoiceParticipantLeft: &eventsv1.VoiceParticipantLeft{RoomId: uuid.NewString(), ProfileId: uuid.NewString()}}
		} else {
			envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantJoined{VoiceParticipantJoined: &eventsv1.VoiceParticipantJoined{RoomId: uuid.NewString(), ProfileId: uuid.NewString()}}
		}
		payload, err := proto.Marshal(envelope)
		require.NoError(t, err)
		profiles, fanout, ok := voiceEventBytesToFanout(payload)
		require.False(t, ok)
		require.Empty(t, profiles)
		require.Empty(t, fanout.Op)
		message := &nats.Msg{Data: payload}
		acks := 0
		consumeVoiceEventMessage(message, nil, nil, func(got *nats.Msg) error { require.Same(t, message, got); acks++; return nil })
		require.Equal(t, 1, acks, "unsupported participant fact still acknowledges once, with no hub access")
	}
}
