package main

import (
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
)

type participantAckSpy struct{ ack, nak, term int }

func (s *participantAckSpy) Ack(...nats.AckOpt) error                         { s.ack++; return nil }
func (s *participantAckSpy) Nak(...nats.AckOpt) error                         { s.nak++; return nil }
func (s *participantAckSpy) NakWithDelay(time.Duration, ...nats.AckOpt) error { s.nak++; return nil }
func (s *participantAckSpy) Term(...nats.AckOpt) error                        { s.term++; return nil }

func TestVoiceParticipantFactsAckWithoutNotificationEffects(t *testing.T) {
	for _, left := range []bool{false, true} {
		envelope := &eventsv1.VoiceStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now()}
		if left {
			envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantLeft{VoiceParticipantLeft: &eventsv1.VoiceParticipantLeft{RoomId: uuid.NewString(), ProfileId: uuid.NewString()}}
		} else {
			envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantJoined{VoiceParticipantJoined: &eventsv1.VoiceParticipantJoined{RoomId: uuid.NewString(), ProfileId: uuid.NewString()}}
		}
		calls := 0
		handler := &consumer.VoiceEventHandler{Router: func(delivery.DeliveryInput) delivery.DeliveryDecision { calls++; return delivery.DeliveryDecision{} }}
		route, err := routeVoiceNotification(t.Context(), handler, nil, nil, envelope, func(string) bool { calls++; return false })
		require.NoError(t, err)
		require.Nil(t, route)
		require.Zero(t, calls, "unsupported facts must not consult presence/router or construct push")
		ack := &participantAckSpy{}
		consumer.JetStreamConsumeAck(ack, err)
		require.Equal(t, 1, ack.ack)
		require.Zero(t, ack.nak)
		require.Zero(t, ack.term)
	}
}
