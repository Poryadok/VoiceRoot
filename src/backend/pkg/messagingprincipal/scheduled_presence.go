// Package messagingprincipal validates the narrowly scoped request body used
// by Messaging's scheduled-delivery presence decision.
package messagingprincipal

import (
	"errors"
	"strings"

	"github.com/google/uuid"
	chatv1 "voice.app/voice/chat/v1"
	userv1 "voice.app/voice/user/v1"
)

var ErrInvalidScheduledPresenceRequest = errors.New("invalid scheduled presence request")

// ScheduledPresenceRequest contains parsed, non-zero identifiers from the
// signed User RPC body. The caller still verifies the service principal and
// current profile/privacy authority before using these IDs.
type ScheduledPresenceRequest struct {
	ScheduledMessageID uuid.UUID
	ScheduleGeneration uint64
	SenderAccountID    uuid.UUID
	SenderProfileID    uuid.UUID
	ChatID             uuid.UUID
	RecipientProfileID uuid.UUID
	Mode               userv1.ScheduledMessageDispatchMode
	ChatType           chatv1.ChatType
}

// ValidateScheduledPresenceRequest rejects an unbound dispatch decision and
// requires canonical UUID encodings so a single request has one body form.
func ValidateScheduledPresenceRequest(req *userv1.GetScheduledMessageDispatchPresenceRequest) (ScheduledPresenceRequest, error) {
	if req == nil || req.GetScheduleGeneration() == 0 {
		return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
	}
	mode := req.GetMode()
	if mode != userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT && mode != userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_WHEN_ONLINE {
		return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
	}
	chatType := req.GetChatType()
	if chatType != chatv1.ChatType_CHAT_TYPE_DM && chatType != chatv1.ChatType_CHAT_TYPE_GROUP && chatType != chatv1.ChatType_CHAT_TYPE_CHANNEL {
		return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
	}
	if mode == userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_WHEN_ONLINE && chatType != chatv1.ChatType_CHAT_TYPE_DM {
		return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
	}
	if chatType == chatv1.ChatType_CHAT_TYPE_DM && req.RecipientProfileId == nil || chatType != chatv1.ChatType_CHAT_TYPE_DM && req.RecipientProfileId != nil {
		return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
	}
	if chatType != chatv1.ChatType_CHAT_TYPE_DM {
		if req.ScheduledMessageId == "" || req.SenderAccountId == "" || req.SenderProfileId == "" || req.ChatId == "" {
			return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
		}
	}
	ids := []struct {
		value string
		dest  *uuid.UUID
	}{
		{req.GetScheduledMessageId(), new(uuid.UUID)},
		{req.GetSenderAccountId(), new(uuid.UUID)},
		{req.GetSenderProfileId(), new(uuid.UUID)},
		{req.GetChatId(), new(uuid.UUID)},
	}
	if chatType == chatv1.ChatType_CHAT_TYPE_DM {
		ids = append(ids, struct {
			value string
			dest  *uuid.UUID
		}{req.GetRecipientProfileId(), new(uuid.UUID)})
	}
	for _, item := range ids {
		value := strings.TrimSpace(item.value)
		parsed, err := uuid.Parse(value)
		if err != nil || parsed == uuid.Nil || value != item.value || parsed.String() != item.value {
			return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
		}
		*item.dest = parsed
	}
	out := ScheduledPresenceRequest{
		ScheduledMessageID: *ids[0].dest,
		ScheduleGeneration: req.GetScheduleGeneration(),
		SenderAccountID:    *ids[1].dest,
		SenderProfileID:    *ids[2].dest,
		ChatID:             *ids[3].dest,
		Mode:               mode,
		ChatType:           chatType,
	}
	if chatType == chatv1.ChatType_CHAT_TYPE_DM {
		out.RecipientProfileID = *ids[4].dest
		if out.SenderProfileID == out.RecipientProfileID {
			return ScheduledPresenceRequest{}, ErrInvalidScheduledPresenceRequest
		}
	}
	return out, nil
}
