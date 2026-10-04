package main

import (
	"context"

	"github.com/google/uuid"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/notification/internal/dispatch"
	"voice/backend/notification/internal/push"
	"voice/backend/notification/internal/store"
)

func socialFriendRequestRestartSpec() notificationConsumerRestartSpec {
	requesterID, recipientID := uuid.New(), uuid.New()
	requestID := uuid.NewString()
	return notificationConsumerRestartSpec{
		name: "social_friend_request", service: "social", stream: jsStreamSocialEvents, filter: "social.>",
		deliverSubject: "_INBOX.voice.notification.social", subject: "social.friend_request",
		recipientID: recipientID,
		event: &eventsv1.SocialStreamEvent{Payload: &eventsv1.SocialStreamEvent_FriendRequest{FriendRequest: &eventsv1.FriendRequest{
			RequestId: requestID, RequesterProfileId: requesterID.String(), TargetProfileId: recipientID.String(),
		}}},
		expected: push.Payload{Title: "Friend request", Body: "You have a new friend request", Data: map[string]string{
			"type": "friend_request", "friend_request_id": requestID, "sender_profile_id": requesterID.String(),
		}},
		run: func(ctx context.Context, natsURL string, tokens *store.DeviceTokenStore, pusher *dispatch.PushDispatcher) error {
			return runSocialEventsConsumer(ctx, natsURL, tokens, pusher, nil, nil, nil)
		},
	}
}
