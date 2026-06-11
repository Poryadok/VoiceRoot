package consumer_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"voice/backend/notification/internal/consumer"
	"voice/backend/notification/internal/delivery"
	eventsv1 "voice.app/voice/events/v1"
)

func TestHandleMessageSent_UsesDefaultRouter(t *testing.T) {
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	h := &consumer.MessageEventHandler{}
	ev := &eventsv1.MessageSent{
		MessageId:       uuid.NewString(),
		ChatId:          uuid.NewString(),
		SenderProfileId: senderID,
	}
	decisions := h.HandleMessageSent(context.Background(), ev, []string{recipientID})
	require.True(t, decisions[recipientID].Push)
}

func TestHandleMessageSent_NilEvent(t *testing.T) {
	h := &consumer.MessageEventHandler{}
	require.Nil(t, h.HandleMessageSent(context.Background(), nil, []string{"p1"}))
}

func TestHandleMentionAdded_NilEvent(t *testing.T) {
	h := &consumer.MessageEventHandler{}
	require.Nil(t, h.HandleMentionAdded(context.Background(), nil))
}

func TestHandleMessageSent_SkipsEmptyAndSenderProfiles(t *testing.T) {
	senderID := uuid.NewString()
	h := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	ev := &eventsv1.MessageSent{
		MessageId:       uuid.NewString(),
		ChatId:          uuid.NewString(),
		SenderProfileId: senderID,
	}
	decisions := h.HandleMessageSent(context.Background(), ev, []string{"", senderID})
	require.Empty(t, decisions)
}

func TestHandleMessageSent_OfflineRecipientGetsPush(t *testing.T) {
	senderID := uuid.NewString()
	recipientID := uuid.NewString()
	otherID := uuid.NewString()

	h := &consumer.MessageEventHandler{
		Router: delivery.DecideRouting,
	}
	ev := &eventsv1.MessageSent{
		MessageId:       uuid.NewString(),
		ChatId:          uuid.NewString(),
		SenderProfileId: senderID,
	}
	decisions := h.HandleMessageSent(context.Background(), ev, []string{senderID, recipientID, otherID})
	require.NotNil(t, decisions)

	recipient, ok := decisions[recipientID]
	require.True(t, ok)
	require.True(t, recipient.InApp)
	require.True(t, recipient.Push, "offline member should get push for MessageSent")

	_, senderNotified := decisions[senderID]
	require.False(t, senderNotified, "sender excluded from MessageSent notifications")
}

func TestHandleMentionAdded_SkipsSenderMention(t *testing.T) {
	senderID := uuid.NewString()
	h := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	ev := &eventsv1.MentionAdded{
		MessageId:           uuid.NewString(),
		ChatId:              uuid.NewString(),
		SenderProfileId:     senderID,
		MentionedProfileIds: []string{senderID},
	}
	decisions := h.HandleMentionAdded(context.Background(), ev)
	require.Empty(t, decisions)
}

func TestHandleMentionAdded_MentionedProfileGetsPush(t *testing.T) {
	senderID := uuid.NewString()
	mentionedID := uuid.NewString()

	h := &consumer.MessageEventHandler{
		Router: delivery.DecideRouting,
	}
	ev := &eventsv1.MentionAdded{
		MessageId:            uuid.NewString(),
		ChatId:               uuid.NewString(),
		SenderProfileId:      senderID,
		MentionedProfileIds:  []string{mentionedID},
	}
	decisions := h.HandleMentionAdded(context.Background(), ev)
	require.NotNil(t, decisions)

	mention, ok := decisions[mentionedID]
	require.True(t, ok)
	require.True(t, mention.Push, "mentioned profile receives push when offline routing applies")
	require.True(t, mention.InApp)
}
