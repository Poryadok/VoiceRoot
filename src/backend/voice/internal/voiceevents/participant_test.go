package voiceevents

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

func participantFixture() ParticipantEvent {
	return ParticipantEvent{ID: uuid.MustParse("11111111-1111-4111-8111-111111111111"), Kind: "participant_joined", RoomID: uuid.MustParse("22222222-2222-4222-8222-222222222222"), ProfileID: uuid.MustParse("33333333-3333-4333-8333-333333333333"), OccurredAt: time.Date(2026, 10, 18, 10, 20, 30, 123456789, time.FixedZone("source", 3600))}
}

func TestParticipantEncoderPreservesCommittedIdentityAndTime(t *testing.T) {
	for _, kind := range []string{"participant_joined", "participant_left"} {
		t.Run(kind, func(t *testing.T) {
			input := participantFixture()
			input.Kind = kind
			encoded, err := EncodeParticipantEvent(input)
			require.NoError(t, err)
			retry, err := EncodeParticipantEvent(input)
			require.NoError(t, err)
			require.Equal(t, encoded, retry)
			require.Equal(t, input.ID, encoded.ID)
			require.Equal(t, "voice."+kind, encoded.Subject)
			var envelope eventsv1.VoiceStreamEvent
			require.NoError(t, proto.Unmarshal(encoded.Payload, &envelope))
			require.Equal(t, input.ID.String(), envelope.EventId)
			require.NoError(t, envelope.OccurredAt.CheckValid())
			require.True(t, input.OccurredAt.Equal(envelope.OccurredAt.AsTime()))
			require.EqualValues(t, 123456789, envelope.OccurredAt.Nanos)
			if kind == "participant_joined" {
				require.Equal(t, input.RoomID.String(), envelope.GetVoiceParticipantJoined().RoomId)
				require.Equal(t, input.ProfileID.String(), envelope.GetVoiceParticipantJoined().ProfileId)
				require.Nil(t, envelope.GetVoiceParticipantLeft())
			} else {
				require.Equal(t, input.RoomID.String(), envelope.GetVoiceParticipantLeft().RoomId)
				require.Equal(t, input.ProfileID.String(), envelope.GetVoiceParticipantLeft().ProfileId)
				require.Nil(t, envelope.GetVoiceParticipantJoined())
			}
			require.Nil(t, envelope.GetVoiceMemberJoined(), "participant fact is not a legacy member notification")
			js := &recordingJetStream{}
			publisher := &JetStreamPublisher{js: js}
			require.NoError(t, publisher.PublishAdmissionEvent(context.Background(), encoded.ID, encoded.Subject, encoded.Payload))
			require.Len(t, js.messages, 1)
			require.Equal(t, encoded.Subject, js.messages[0].Subject)
			require.Equal(t, encoded.Payload, js.messages[0].Data)
			require.Equal(t, input.ID.String(), js.messages[0].Header.Get("Nats-Msg-Id"))
		})
	}
}

func TestParticipantEncoderRejectsInvalidCommittedFacts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*ParticipantEvent)
	}{
		{"event ID", func(e *ParticipantEvent) { e.ID = uuid.Nil }}, {"room ID", func(e *ParticipantEvent) { e.RoomID = uuid.Nil }}, {"profile ID", func(e *ParticipantEvent) { e.ProfileID = uuid.Nil }},
		{"foreign kind", func(e *ParticipantEvent) { e.Kind = "voice.member_joined" }}, {"blank kind", func(e *ParticipantEvent) { e.Kind = "" }},
		{"year zero", func(e *ParticipantEvent) { e.OccurredAt = time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC) }}, {"year10000", func(e *ParticipantEvent) { e.OccurredAt = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := participantFixture()
			tc.mutate(&input)
			encoded, err := EncodeParticipantEvent(input)
			require.Error(t, err)
			require.Empty(t, encoded.Payload)
			require.Equal(t, uuid.Nil, encoded.ID)
		})
	}
}

func TestParticipantDurablePublisherRejectsSubjectPayloadMismatch(t *testing.T) {
	for _, subject := range []string{"voice.participant_joined", "voice.participant_left"} {
		input := participantFixture()
		if subject == "voice.participant_left" {
			input.Kind = "participant_left"
		}
		encoded, err := EncodeParticipantEvent(input)
		require.NoError(t, err)
		js := &recordingJetStream{}
		publisher := &JetStreamPublisher{js: js}
		other := "voice.participant_left"
		if subject == other {
			other = "voice.participant_joined"
		}
		require.Error(t, publisher.PublishAdmissionEvent(context.Background(), encoded.ID, other, encoded.Payload))
		require.Error(t, publisher.PublishAdmissionEvent(context.Background(), uuid.New(), subject, encoded.Payload))
		require.Empty(t, js.messages, "invalid durable bindings never reach NATS")
	}
}

func TestParticipantDurablePublisherRejectsInvalidPayload(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*eventsv1.VoiceStreamEvent)
	}{
		{"missing occurrence", func(e *eventsv1.VoiceStreamEvent) { e.OccurredAt = nil }},
		{"invalid occurrence", func(e *eventsv1.VoiceStreamEvent) { e.OccurredAt.Nanos = 1000000000 }},
		{"bad room", func(e *eventsv1.VoiceStreamEvent) { e.GetVoiceParticipantJoined().RoomId = "not-a-uuid" }},
		{"nil profile", func(e *eventsv1.VoiceStreamEvent) { e.GetVoiceParticipantJoined().ProfileId = uuid.Nil.String() }},
		{"missing body", func(e *eventsv1.VoiceStreamEvent) { e.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantJoined{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := EncodeParticipantEvent(participantFixture())
			require.NoError(t, err)
			var envelope eventsv1.VoiceStreamEvent
			require.NoError(t, proto.Unmarshal(encoded.Payload, &envelope))
			tc.mutate(&envelope)
			payload, err := proto.Marshal(&envelope)
			require.NoError(t, err)
			js := &recordingJetStream{}
			publisher := &JetStreamPublisher{js: js}
			require.Error(t, publisher.PublishAdmissionEvent(context.Background(), encoded.ID, encoded.Subject, payload))
			require.Empty(t, js.messages)
		})
	}
}
