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

func TestHandleFriendRequest_TargetGetsPush(t *testing.T) {
	requester := uuid.NewString()
	target := uuid.NewString()
	h := &consumer.SocialEventHandler{Router: delivery.DecideRouting}
	decisions := h.HandleFriendRequest(context.Background(), &eventsv1.FriendRequest{
		RequestId:           uuid.NewString(),
		RequesterProfileId:  requester,
		TargetProfileId:     target,
	})
	require.True(t, decisions[target].Push)
}

func TestHandleMessageReply_OnlyParentAuthor(t *testing.T) {
	senderID := uuid.NewString()
	parentAuthor := uuid.NewString()
	h := &consumer.MessageEventHandler{Router: delivery.DecideRouting}
	ev := &eventsv1.MessageSent{
		MessageId:       uuid.NewString(),
		ChatId:          uuid.NewString(),
		SenderProfileId: senderID,
		ThreadParentId:  ptr(uuid.NewString()),
	}
	decisions := h.HandleMessageReply(context.Background(), ev, parentAuthor)
	require.Len(t, decisions, 1)
	require.True(t, decisions[parentAuthor].Push)
}

func ptr(s string) *string { return &s }
