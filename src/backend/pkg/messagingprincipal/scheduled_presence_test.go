package messagingprincipal

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	userv1 "voice.app/voice/user/v1"
)

func TestValidateScheduledPresenceRequestRequiresCanonicalBoundIDsGenerationAndMode(t *testing.T) {
	ids := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	peer := ids[4].String()
	valid := &userv1.GetScheduledMessageDispatchPresenceRequest{
		ScheduledMessageId: ids[0].String(),
		SenderAccountId:    ids[1].String(),
		SenderProfileId:    ids[2].String(),
		ChatId:             ids[3].String(),
		RecipientProfileId: &peer,
		ScheduleGeneration: 3,
		Mode:               userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_WHEN_ONLINE,
		ChatType:           chatv1.ChatType_CHAT_TYPE_DM,
	}
	got, err := ValidateScheduledPresenceRequest(valid)
	require.NoError(t, err)
	require.Equal(t, ids, []uuid.UUID{got.ScheduledMessageID, got.SenderAccountID, got.SenderProfileID, got.ChatID, got.RecipientProfileID})
	require.Equal(t, uint64(3), got.ScheduleGeneration)
	require.Equal(t, valid.GetMode(), got.Mode)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, got.ChatType)
	nilRecipient := uuid.Nil.String()

	for name, mutate := range map[string]func(*userv1.GetScheduledMessageDispatchPresenceRequest){
		"nil":                 nil,
		"missing schedule id": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) { r.ScheduledMessageId = "" },
		"noncanonical id":     func(r *userv1.GetScheduledMessageDispatchPresenceRequest) { r.ChatId = " " + r.ChatId },
		"nil recipient":       func(r *userv1.GetScheduledMessageDispatchPresenceRequest) { r.RecipientProfileId = &nilRecipient },
		"same sender recipient": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			recipient := r.SenderProfileId
			r.RecipientProfileId = &recipient
		},
		"zero generation": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) { r.ScheduleGeneration = 0 },
		"unspecified mode": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.Mode = userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_UNSPECIFIED
		},
		"unknown mode": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.Mode = userv1.ScheduledMessageDispatchMode(77)
		},
		"unspecified chat type": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.ChatType = chatv1.ChatType_CHAT_TYPE_UNSPECIFIED
		},
		"unknown chat type": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.ChatType = chatv1.ChatType(77)
		},
		"DM missing peer": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.RecipientProfileId = nil
		},
		"non-DM with peer": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.ChatType = chatv1.ChatType_CHAT_TYPE_GROUP
			r.Mode = userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT
		},
		"online mode non-DM": func(r *userv1.GetScheduledMessageDispatchPresenceRequest) {
			r.ChatType = chatv1.ChatType_CHAT_TYPE_GROUP
			r.RecipientProfileId = nil
		},
	} {
		t.Run(name, func(t *testing.T) {
			request := protoCloneScheduledPresenceRequest(valid)
			if mutate == nil {
				request = nil
			} else {
				mutate(request)
			}
			_, err := ValidateScheduledPresenceRequest(request)
			require.ErrorIs(t, err, ErrInvalidScheduledPresenceRequest)
		})
	}
}

func protoCloneScheduledPresenceRequest(in *userv1.GetScheduledMessageDispatchPresenceRequest) *userv1.GetScheduledMessageDispatchPresenceRequest {
	var recipientProfileID *string
	if in.RecipientProfileId != nil {
		value := in.GetRecipientProfileId()
		recipientProfileID = &value
	}
	return &userv1.GetScheduledMessageDispatchPresenceRequest{
		ScheduledMessageId: in.GetScheduledMessageId(),
		SenderAccountId:    in.GetSenderAccountId(),
		SenderProfileId:    in.GetSenderProfileId(),
		ChatId:             in.GetChatId(),
		RecipientProfileId: recipientProfileID,
		ScheduleGeneration: in.GetScheduleGeneration(),
		Mode:               in.GetMode(),
		ChatType:           in.GetChatType(),
	}
}

func TestValidateScheduledPresenceRequestAllowsNonDMAtOnlyWithoutPeer(t *testing.T) {
	request := &userv1.GetScheduledMessageDispatchPresenceRequest{
		ScheduledMessageId: uuid.NewString(), SenderAccountId: uuid.NewString(), SenderProfileId: uuid.NewString(),
		ChatId: uuid.NewString(), ScheduleGeneration: 9,
		Mode:     userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT,
		ChatType: chatv1.ChatType_CHAT_TYPE_CHANNEL,
	}
	parsed, err := ValidateScheduledPresenceRequest(request)
	require.NoError(t, err)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_CHANNEL, parsed.ChatType)
	require.Equal(t, uuid.Nil, parsed.RecipientProfileID)
	require.Equal(t, userv1.ScheduledMessageDispatchMode_SCHEDULED_MESSAGE_DISPATCH_MODE_AT, parsed.Mode)
}
