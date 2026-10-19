package grpcsvc

import (
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"math"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/voice/internal/ordinarycall"
	"voice/backend/voice/internal/voiceevents"
)

// EncodeOrdinaryEvent runs inside the native transaction. No identity, time or
// recipient is generated during delivery; retries use these exact saved bytes.
func EncodeOrdinaryEvent(r *ordinarycall.Room, e ordinarycall.Event) (ordinarycall.EncodedEvent, error) {
	invalid := ordinarycall.ErrInvalidIntent
	if r == nil || e.ID == uuid.Nil || e.OccurredAt.IsZero() || r.Intent.RoomID == uuid.Nil {
		return ordinarycall.EncodedEvent{}, invalid
	}
	if e.Kind == ordinarycall.EventParticipantJoined || e.Kind == ordinarycall.EventParticipantLeft {
		encoded, err := voiceevents.EncodeParticipantEvent(voiceevents.ParticipantEvent{ID: e.ID, Kind: string(e.Kind), RoomID: r.Intent.RoomID, ProfileID: e.ProfileID, OccurredAt: e.OccurredAt})
		return ordinarycall.EncodedEvent{ID: encoded.ID, Subject: encoded.Subject, Payload: encoded.Payload}, err
	}
	at := timestamppb.New(e.OccurredAt)
	if at.CheckValid() != nil {
		return ordinarycall.EncodedEvent{}, invalid
	}
	env := &eventsv1.VoiceStreamEvent{EventId: e.ID.String(), OccurredAt: at}
	room, chat, initiator := r.Intent.RoomID.String(), r.Intent.ChatID.String(), r.Intent.Initiator.ProfileID.String()
	callee := ""
	if r.Intent.Callee != nil {
		callee = r.Intent.Callee.ProfileID.String()
	}
	audience := make([]string, 0, len(e.AudienceIDs))
	for _, p := range e.AudienceIDs {
		if p == uuid.Nil {
			return ordinarycall.EncodedEvent{}, invalid
		}
		audience = append(audience, p.String())
	}
	by := ""
	if e.ActorID != uuid.Nil {
		by = e.ActorID.String()
	}
	media := "audio"
	if r.Intent.MediaKind == 2 {
		media = "video"
	}
	subject := "voice.call_" + string(e.Kind)
	switch e.Kind {
	case ordinarycall.EventIncoming:
		expiry := timestamppb.New(r.ExpiresAt)
		if expiry.CheckValid() != nil || r.ExpiresAt.IsZero() {
			return ordinarycall.EncodedEvent{}, invalid
		}
		env.Payload = &eventsv1.VoiceStreamEvent_CallIncoming{CallIncoming: &eventsv1.CallIncoming{RoomId: room, ChatId: chat, InitiatorProfileId: initiator, CalleeProfileId: callee, MediaKind: media, LivekitRoomName: r.LiveKitRoomName, ExpiresAt: expiry}}
	case ordinarycall.EventAccepted:
		env.Payload = &eventsv1.VoiceStreamEvent_CallAccepted{CallAccepted: &eventsv1.CallAccepted{RoomId: room, ChatId: chat, AcceptedByProfileId: by, ProfileIds: audience, MediaKind: media, LivekitRoomName: r.LiveKitRoomName}}
	case ordinarycall.EventDeclined:
		env.Payload = &eventsv1.VoiceStreamEvent_CallDeclined{CallDeclined: &eventsv1.CallDeclined{RoomId: room, ChatId: chat, DeclinedByProfileId: by, ProfileIds: audience}}
	case ordinarycall.EventMissed:
		env.Payload = &eventsv1.VoiceStreamEvent_CallMissed{CallMissed: &eventsv1.CallMissed{RoomId: room, ChatId: chat, InitiatorProfileId: initiator, CalleeProfileId: callee}}
	case ordinarycall.EventEnded:
		seconds := e.OccurredAt.Sub(r.ReadyAt).Seconds()
		if r.ReadyAt.IsZero() || seconds < 0 {
			seconds = 0
		}
		if seconds > math.MaxInt32 {
			seconds = math.MaxInt32
		}
		env.Payload = &eventsv1.VoiceStreamEvent_CallEnded{CallEnded: &eventsv1.CallEnded{RoomId: room, DurationSeconds: int32(seconds), ProfileIds: audience, Reason: e.Reason, EndedByProfileId: by}}
	case ordinarycall.EventCallStarted:
		subject = "voice.call_started"
		kind := "call"
		if r.Intent.Group {
			kind = "group_voice"
		}
		env.Payload = &eventsv1.VoiceStreamEvent_CallStarted{CallStarted: &eventsv1.CallStarted{RoomId: room, ProfileIds: audience, ChatId: chat, InitiatorProfileId: initiator, CalleeProfileId: callee, MediaKind: media, LivekitRoomName: r.LiveKitRoomName, RoomType: &kind}}
	default:
		return ordinarycall.EncodedEvent{}, invalid
	}
	payload, err := proto.MarshalOptions{Deterministic: true}.Marshal(env)
	return ordinarycall.EncodedEvent{ID: e.ID, Subject: subject, Payload: payload}, err
}
