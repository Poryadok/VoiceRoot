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

func TestDeliveryAckFromEventRejectsNonDeliveryAndMalformedPayloads(t *testing.T) {
	validID := uuid.NewString()
	tests := []struct {
		name string
		env  *eventsv1.MessageStreamEvent
		raw  []byte
	}{
		{
			name: "other message event",
			env: &eventsv1.MessageStreamEvent{
				EventId: uuid.NewString(),
				Payload: &eventsv1.MessageStreamEvent_MessageRead{
					MessageRead: &eventsv1.MessageRead{ChatId: validID, ProfileId: validID, MessageId: validID},
				},
			},
		},
		{
			name: "malformed delivery acknowledgement id",
			env: &eventsv1.MessageStreamEvent{
				EventId: uuid.NewString(),
				Payload: &eventsv1.MessageStreamEvent_DeliveryAck{
					DeliveryAck: &eventsv1.DeliveryAck{ChatId: "not-a-uuid", ProfileId: validID, MessageId: validID},
				},
			},
		},
		{name: "invalid protobuf", raw: []byte("not-protobuf")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := tt.raw
			if tt.env != nil {
				var err error
				raw, err = proto.Marshal(tt.env)
				require.NoError(t, err)
			}
			chatID, profileID, messageID, ok := deliveryAckFromEvent(raw)
			require.False(t, ok)
			require.Equal(t, uuid.Nil, chatID)
			require.Equal(t, uuid.Nil, profileID)
			require.Equal(t, uuid.Nil, messageID)
		})
	}
}
