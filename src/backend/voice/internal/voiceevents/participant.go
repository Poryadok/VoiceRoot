package voiceevents

import (
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	eventsv1 "voice.app/voice/events/v1"
)

// ParticipantEvent is an already committed membership fact from the ordinary
// call journal. Encoding never elects a transition or creates an event identity.
type ParticipantEvent struct {
	ID                uuid.UUID
	Kind              string
	RoomID, ProfileID uuid.UUID
	OccurredAt        time.Time
}

// EncodedParticipantEvent is persisted with the committed journal transition.
type EncodedParticipantEvent struct {
	ID      uuid.UUID
	Subject string
	Payload []byte
}

// EncodeParticipantEvent freezes the exact NATS-only protobuf envelope before
// the outbox transaction commits. Retries publish these saved bytes unchanged.
func EncodeParticipantEvent(event ParticipantEvent) (EncodedParticipantEvent, error) {
	invalid := errors.New("invalid committed participant event")
	if event.ID == uuid.Nil || event.RoomID == uuid.Nil || event.ProfileID == uuid.Nil {
		return EncodedParticipantEvent{}, invalid
	}
	at := timestamppb.New(event.OccurredAt)
	if at.CheckValid() != nil {
		return EncodedParticipantEvent{}, invalid
	}
	envelope := &eventsv1.VoiceStreamEvent{EventId: event.ID.String(), OccurredAt: at}
	switch event.Kind {
	case "participant_joined":
		envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantJoined{VoiceParticipantJoined: &eventsv1.VoiceParticipantJoined{RoomId: event.RoomID.String(), ProfileId: event.ProfileID.String()}}
	case "participant_left":
		envelope.Payload = &eventsv1.VoiceStreamEvent_VoiceParticipantLeft{VoiceParticipantLeft: &eventsv1.VoiceParticipantLeft{RoomId: event.RoomID.String(), ProfileId: event.ProfileID.String()}}
	default:
		return EncodedParticipantEvent{}, invalid
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
	if err != nil {
		return EncodedParticipantEvent{}, err
	}
	return EncodedParticipantEvent{ID: event.ID, Subject: "voice." + event.Kind, Payload: payload}, nil
}
