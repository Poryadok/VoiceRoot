package grpcsvc

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

func TestVoiceGRPCRaiseHandAndCommanderMode(t *testing.T) {
	events := &recordingEvents{}
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), events)
	group := chatv1.ChatType_CHAT_TYPE_GROUP

	start, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)
	roomID := start.GetCallSession().GetRoomId()

	_, err = svc.RaiseHand(voiceTestCtx("profile-owner"), &callsv1.RaiseHandRequest{RoomId: roomID})
	require.NoError(t, err)

	states, err := svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	ownerState := findParticipantState(states.GetParticipants(), "profile-owner")
	require.True(t, ownerState.GetHandRaised())

	_, err = svc.SetCommanderMode(voiceTestCtx("profile-owner"), &callsv1.SetCommanderModeRequest{
		RoomId:  roomID,
		Enabled: true,
	})
	require.NoError(t, err)

	states, err = svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	ownerState = findParticipantState(states.GetParticipants(), "profile-owner")
	require.True(t, ownerState.GetIsCommander())

	_, err = svc.LowerHand(voiceTestCtx("profile-owner"), &callsv1.LowerHandRequest{RoomId: roomID})
	require.NoError(t, err)

	states, err = svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	ownerState = findParticipantState(states.GetParticipants(), "profile-owner")
	require.False(t, ownerState.GetHandRaised())
}

func findParticipantState(states []*callsv1.VoiceParticipantState, profileID string) *callsv1.VoiceParticipantState {
	for _, s := range states {
		if s.GetProfileId() == profileID {
			return s
		}
	}
	return &callsv1.VoiceParticipantState{}
}
