package main

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func TestFriendRequestEventToFanout_TargetOnly(t *testing.T) {
	requestID, requesterID, targetID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	payload, err := proto.Marshal(&eventsv1.SocialStreamEvent{Payload: &eventsv1.SocialStreamEvent_FriendRequest{
		FriendRequest: &eventsv1.FriendRequest{
			RequestId: requestID, RequesterProfileId: requesterID, TargetProfileId: targetID,
		},
	}})
	require.NoError(t, err)

	profileID, envelope, ok := friendRequestEventToFanout(payload)
	require.True(t, ok)
	require.Equal(t, targetID, profileID)
	require.Equal(t, map[string]string{
		"type": "friend_request", "friend_request_id": requestID, "sender_profile_id": requesterID,
	}, notificationPayload(t, envelope))
}

func TestFriendRequestEventToFanout_RejectsMissingTarget(t *testing.T) {
	payload, err := proto.Marshal(&eventsv1.SocialStreamEvent{Payload: &eventsv1.SocialStreamEvent_FriendRequest{
		FriendRequest: &eventsv1.FriendRequest{RequestId: uuid.NewString(), RequesterProfileId: uuid.NewString()},
	}})
	require.NoError(t, err)
	_, _, ok := friendRequestEventToFanout(payload)
	require.False(t, ok)
}
