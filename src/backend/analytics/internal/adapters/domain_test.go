package adapters

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestMapperFromMessageSent(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	ev := m.FromMessage(&eventsv1.MessageStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_MessageSent{
			MessageSent: &eventsv1.MessageSent{
				ChatId:          "chat-1",
				MessageId:       "msg-1",
				SenderProfileId: "profile-1",
			},
		},
	})
	require.NotNil(t, ev)
	require.Equal(t, "message_sent", ev.GetEventType())
	require.Equal(t, "messaging", ev.GetSourceService())
	require.NotEmpty(t, ev.GetProfileIdHashed())
}

func TestMapperFromUserRegistered(t *testing.T) {
	m := Mapper{HashKey: "test-key"}
	ev := m.FromUser(&eventsv1.UserStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.UserStreamEvent_UserRegistered{
			UserRegistered: &eventsv1.UserRegistered{
				AccountId: "acc-1",
				Type:      "user",
				Method:    "email",
			},
		},
	})
	require.NotNil(t, ev)
	require.Equal(t, "user_registered", ev.GetEventType())
	require.NotEmpty(t, ev.GetUserIdHashed())
}
