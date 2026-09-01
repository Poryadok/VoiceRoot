package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestDeliveryAckFromEvent(t *testing.T) {
	chatID := uuid.NewString()
	profileID := uuid.NewString()
	messageID := uuid.NewString()
	env := &eventsv1.MessageStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MessageStreamEvent_DeliveryAck{
			DeliveryAck: &eventsv1.DeliveryAck{
				ChatId:    chatID,
				ProfileId: profileID,
				MessageId: messageID,
			},
		},
	}
	b, err := proto.Marshal(env)
	require.NoError(t, err)
	gotChat, gotProfile, gotMsg, ok := deliveryAckFromEvent(b)
	require.True(t, ok)
	require.Equal(t, chatID, gotChat.String())
	require.Equal(t, profileID, gotProfile.String())
	require.Equal(t, messageID, gotMsg.String())
}
