package main

import (
	"encoding/json"
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

func TestFriendRemovedEventToFanout_NotifiesBothProfilesWithPeerID(t *testing.T) {
	profileA, profileB := uuid.NewString(), uuid.NewString()
	payload, err := proto.Marshal(&eventsv1.SocialStreamEvent{Payload: &eventsv1.SocialStreamEvent_FriendRemoved{
		FriendRemoved: &eventsv1.FriendRemoved{ProfileIdA: profileA, ProfileIdB: profileB},
	}})
	require.NoError(t, err)

	fanouts, ok := friendRemovedEventToFanout(payload)
	require.True(t, ok)
	require.Equal(t, []profileFanout{
		{ProfileID: profileA, Envelope: friendRemovedEnvelope(t, profileB)},
		{ProfileID: profileB, Envelope: friendRemovedEnvelope(t, profileA)},
	}, fanouts)
}

func TestFriendRemovedEventToFanout_RejectsMissingProfile(t *testing.T) {
	payload, err := proto.Marshal(&eventsv1.SocialStreamEvent{Payload: &eventsv1.SocialStreamEvent_FriendRemoved{
		FriendRemoved: &eventsv1.FriendRemoved{ProfileIdA: uuid.NewString()},
	}})
	require.NoError(t, err)
	_, ok := friendRemovedEventToFanout(payload)
	require.False(t, ok)
}

func friendRemovedEnvelope(t *testing.T, peerID string) fanoutEnvelope {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"type": "friend_removed", "friend_profile_id": peerID,
	})
	require.NoError(t, err)
	return fanoutEnvelope{Op: "notification", D: payload}
}
