// Package roleeventwire decodes ordinary Role notifications, independently of
// the dedicated Voice policy invalidation and unspecified generic Role arms.
package roleeventwire

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

var (
	ErrUnsupported = errors.New("unsupported ordinary Role event")
	ErrInvalid     = errors.New("invalid ordinary Role event")
)

// Event preserves notification data. Legacy records deliberately have no
// source event ID/time: consumers must retain their own stream-sequence identity.
type Event struct {
	Subject    string
	Mutation   *eventsv1.RoleMutation
	EventID    string
	OccurredAt *timestamppb.Timestamp
	Legacy     bool
}

var envelopeKeys = func() map[string]bool {
	keys := map[string]bool{"payload": true}
	fields := (&eventsv1.RoleStreamEvent{}).ProtoReflect().Descriptor().Fields()
	for index := 0; index < fields.Len(); index++ {
		field := fields.Get(index)
		keys[string(field.Name())] = true
		keys[field.JSONName()] = true
	}
	return keys
}()

// Decode accepts only the nine ordinary actions. A valid JSON object is the
// historical flat format; typed protobuf always derives its action from oneof.
func Decode(subject string, data []byte) (Event, error) {
	if !ordinarySubject(subject) {
		return Event{}, ErrUnsupported
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed) {
		return decodeLegacy(subject, trimmed)
	}
	var envelope eventsv1.RoleStreamEvent
	if err := proto.Unmarshal(data, &envelope); err != nil {
		return Event{}, ErrInvalid
	}
	var expected string
	var mutation *eventsv1.RoleMutation
	switch payload := envelope.GetPayload().(type) {
	case *eventsv1.RoleStreamEvent_RoleCreated:
		expected, mutation = "role.created", payload.RoleCreated
	case *eventsv1.RoleStreamEvent_RoleUpdated:
		expected, mutation = "role.updated", payload.RoleUpdated
	case *eventsv1.RoleStreamEvent_RoleDeleted:
		expected, mutation = "role.deleted", payload.RoleDeleted
	case *eventsv1.RoleStreamEvent_RoleAssigned:
		expected, mutation = "role.assigned", payload.RoleAssigned
	case *eventsv1.RoleStreamEvent_RoleRevoked:
		expected, mutation = "role.revoked", payload.RoleRevoked
	case *eventsv1.RoleStreamEvent_RoleChatOverrideSet:
		expected, mutation = "role.chat_override_set", payload.RoleChatOverrideSet
	case *eventsv1.RoleStreamEvent_RoleChatOverrideRemoved:
		expected, mutation = "role.chat_override_removed", payload.RoleChatOverrideRemoved
	case *eventsv1.RoleStreamEvent_RoleVoiceOverrideSet:
		expected, mutation = "role.voice_override_set", payload.RoleVoiceOverrideSet
	case *eventsv1.RoleStreamEvent_RoleVoiceOverrideRemoved:
		expected, mutation = "role.voice_override_removed", payload.RoleVoiceOverrideRemoved
	default:
		return Event{}, ErrUnsupported
	}
	if subject != expected || mutation == nil || !requiredData(subject, mutation) {
		return Event{}, ErrInvalid
	}
	id, err := uuid.Parse(envelope.GetEventId())
	if err != nil || id == uuid.Nil || id.String() != envelope.GetEventId() || envelope.GetOccurredAt() == nil || envelope.GetOccurredAt().CheckValid() != nil {
		return Event{}, ErrInvalid
	}
	return Event{Subject: subject, Mutation: mutation, EventID: envelope.GetEventId(), OccurredAt: envelope.GetOccurredAt()}, nil
}

func ordinarySubject(subject string) bool {
	switch subject {
	case "role.created", "role.updated", "role.deleted", "role.assigned", "role.revoked", "role.chat_override_set", "role.chat_override_removed", "role.voice_override_set", "role.voice_override_removed":
		return true
	default:
		return false
	}
}

func requiredData(subject string, mutation *eventsv1.RoleMutation) bool {
	if strings.TrimSpace(mutation.GetSpaceId()) == "" || strings.TrimSpace(mutation.GetRoleId()) == "" {
		return false
	}
	switch subject {
	case "role.assigned", "role.revoked":
		return strings.TrimSpace(mutation.GetProfileId()) != ""
	case "role.chat_override_set", "role.chat_override_removed":
		return strings.TrimSpace(mutation.GetChatId()) != ""
	case "role.voice_override_set", "role.voice_override_removed":
		return strings.TrimSpace(mutation.GetVoiceRoomId()) != ""
	default:
		return true
	}
}

func decodeLegacy(subject string, data []byte) (Event, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil || object == nil {
		return Event{}, ErrInvalid
	}
	identified := false
	for key := range object {
		if envelopeKeys[key] {
			return Event{}, ErrInvalid
		}
		switch key {
		case "space_id", "role_id", "profile_id", "name", "chat_id", "voice_room_id", "changed_fields":
			identified = true
		}
	}
	if !identified {
		return Event{}, ErrInvalid
	}
	var mutation eventsv1.RoleMutation
	if err := json.Unmarshal(data, &mutation); err != nil {
		return Event{}, ErrInvalid
	}
	return Event{Subject: subject, Mutation: &mutation, Legacy: true}, nil
}
