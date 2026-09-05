package grpcsvc

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	voicestore "voice/backend/voice/internal/store"

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

func TestVoiceGRPCGrantFloorAndBroadcasting(t *testing.T) {
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

	_, err = svc.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: roomID})
	require.NoError(t, err)

	_, err = svc.RaiseHand(voiceTestCtx("profile-member"), &callsv1.RaiseHandRequest{RoomId: roomID})
	require.NoError(t, err)

	// Non-organizer cannot grant floor.
	_, err = svc.GrantFloor(voiceTestCtx("profile-member"), &callsv1.GrantFloorRequest{
		RoomId:    roomID,
		ProfileId: "profile-member",
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = svc.SetCommanderMode(voiceTestCtx("profile-owner"), &callsv1.SetCommanderModeRequest{
		RoomId:  roomID,
		Enabled: true,
	})
	require.NoError(t, err)

	_, err = svc.GrantFloor(voiceTestCtx("profile-owner"), &callsv1.GrantFloorRequest{
		RoomId:    roomID,
		ProfileId: "profile-member",
	})
	require.NoError(t, err)

	states, err := svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	member := findParticipantState(states.GetParticipants(), "profile-member")
	require.True(t, member.GetHasFloor())
	require.False(t, member.GetHandRaised())

	_, err = svc.SetBroadcasting(voiceTestCtx("profile-owner"), &callsv1.SetBroadcastingRequest{
		RoomId:  roomID,
		Enabled: true,
	})
	require.NoError(t, err)

	states, err = svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	owner := findParticipantState(states.GetParticipants(), "profile-owner")
	require.True(t, owner.GetIsBroadcasting())

	_, err = svc.RevokeFloor(voiceTestCtx("profile-owner"), &callsv1.RevokeFloorRequest{
		RoomId:    roomID,
		ProfileId: "profile-member",
	})
	require.NoError(t, err)

	states, err = svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	member = findParticipantState(states.GetParticipants(), "profile-member")
	require.False(t, member.GetHasFloor())

	// Non-commander cannot broadcast.
	_, err = svc.SetBroadcasting(voiceTestCtx("profile-member"), &callsv1.SetBroadcastingRequest{
		RoomId:  roomID,
		Enabled: true,
	})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVoiceGRPCVoiceRoom_muteOthersDenialOrUnavailabilityPreventsCommanderAndFloorMutations(t *testing.T) {
	checkers := []struct {
		name string
		err  error
	}{
		{name: "denied", err: errors.New("VOICE_MUTE_OTHERS denied")},
		{name: "unavailable", err: errors.New("role service unavailable")},
	}
	for _, checker := range checkers {
		t.Run(checker.name, func(t *testing.T) {
			t.Run("enable commander mode", func(t *testing.T) {
				f := startVoiceRoomFixture(t)
				roles := &recordingVoiceRolePermissions{muteOthersErr: checker.err}
				f.svc.Roles = roles
				roomID := f.joinParticipants(t)

				_, err := f.svc.SetCommanderMode(voiceTestCtx("profile-owner"), &callsv1.SetCommanderModeRequest{RoomId: roomID, Enabled: true})
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				requireVoiceRoleCheck(t, roles.muteOthersChecks, f.spaceID, "profile-owner", f.voiceRoomID)
				require.False(t, f.participantState(t, roomID, "profile-owner").GetIsCommander(), "denied commander mode must not mutate state")
			})

			t.Run("begin broadcasting", func(t *testing.T) {
				f := startVoiceRoomFixture(t)
				roles := &recordingVoiceRolePermissions{muteOthersErr: checker.err}
				f.svc.Roles = roles
				roomID := f.joinParticipants(t)
				commander := true
				f.setParticipantState(t, roomID, "profile-owner", voicestore.VoiceStatePatch{IsCommander: &commander})

				_, err := f.svc.SetBroadcasting(voiceTestCtx("profile-owner"), &callsv1.SetBroadcastingRequest{RoomId: roomID, Enabled: true})
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				requireVoiceRoleCheck(t, roles.muteOthersChecks, f.spaceID, "profile-owner", f.voiceRoomID)
				require.False(t, f.participantState(t, roomID, "profile-owner").GetIsBroadcasting(), "denied broadcast must not mutate state")
			})

			t.Run("grant floor", func(t *testing.T) {
				f := startVoiceRoomFixture(t)
				roles := &recordingVoiceRolePermissions{muteOthersErr: checker.err}
				f.svc.Roles = roles
				roomID := f.joinParticipants(t)

				_, err := f.svc.GrantFloor(voiceTestCtx("profile-owner"), &callsv1.GrantFloorRequest{RoomId: roomID, ProfileId: "profile-member"})
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				requireVoiceRoleCheck(t, roles.muteOthersChecks, f.spaceID, "profile-owner", f.voiceRoomID)
				require.False(t, f.participantState(t, roomID, "profile-member").GetHasFloor(), "denied floor grant must not mutate state")
			})

			t.Run("revoke floor", func(t *testing.T) {
				f := startVoiceRoomFixture(t)
				roles := &recordingVoiceRolePermissions{muteOthersErr: checker.err}
				f.svc.Roles = roles
				roomID := f.joinParticipants(t)
				hasFloor := true
				f.setParticipantState(t, roomID, "profile-member", voicestore.VoiceStatePatch{HasFloor: &hasFloor})

				_, err := f.svc.RevokeFloor(voiceTestCtx("profile-owner"), &callsv1.RevokeFloorRequest{RoomId: roomID, ProfileId: "profile-member"})
				require.Equal(t, codes.PermissionDenied, status.Code(err))
				requireVoiceRoleCheck(t, roles.muteOthersChecks, f.spaceID, "profile-owner", f.voiceRoomID)
				require.True(t, f.participantState(t, roomID, "profile-member").GetHasFloor(), "denied floor revocation must not mutate state")
			})
		})
	}
}

func TestVoiceGRPCVoiceRoom_broadcastingRequiresVoiceSpeak(t *testing.T) {
	f := startVoiceRoomFixture(t)
	roles := &recordingVoiceRolePermissions{voiceSpeakErr: errors.New("VOICE_SPEAK denied")}
	f.svc.Roles = roles
	roomID := f.joinParticipants(t)
	commander := true
	f.setParticipantState(t, roomID, "profile-owner", voicestore.VoiceStatePatch{IsCommander: &commander})

	_, err := f.svc.SetBroadcasting(voiceTestCtx("profile-owner"), &callsv1.SetBroadcastingRequest{RoomId: roomID, Enabled: true})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	requireVoiceRoleCheck(t, roles.muteOthersChecks, f.spaceID, "profile-owner", f.voiceRoomID)
	requireVoiceRoleCheck(t, roles.voiceSpeakChecks, f.spaceID, "profile-owner", f.voiceRoomID)
	require.False(t, f.participantState(t, roomID, "profile-owner").GetIsBroadcasting(), "denied broadcast must not mutate state")
}

func findParticipantState(states []*callsv1.VoiceParticipantState, profileID string) *callsv1.VoiceParticipantState {
	for _, s := range states {
		if s.GetProfileId() == profileID {
			return s
		}
	}
	return &callsv1.VoiceParticipantState{}
}
