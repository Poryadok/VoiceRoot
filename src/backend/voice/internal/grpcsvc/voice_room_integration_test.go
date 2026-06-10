package grpcsvc

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

type voiceRoomFixture struct {
	svc         *VoiceGRPC
	spaceID     string
	voiceRoomID string
}

func startVoiceRoomFixture(t *testing.T) voiceRoomFixture {
	t.Helper()
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	members := map[string]map[string]bool{
		spaceID: {
			"profile-owner":  true,
			"profile-member": true,
		},
	}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	return voiceRoomFixture{svc: svc, spaceID: spaceID, voiceRoomID: voiceRoomID}
}

func (f voiceRoomFixture) joinReq(profileID string) *callsv1.JoinVoiceRoomRequest {
	return &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: f.voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: f.spaceID},
	}
}

func TestVoiceGRPCJoinVoiceRoom_createsActiveSession(t *testing.T) {
	f := startVoiceRoomFixture(t)

	joined, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)
	session := joined.GetVoiceSession()
	require.NotEmpty(t, session.GetRoomId())
	require.Equal(t, f.voiceRoomID, session.GetVoiceRoomId())
	require.Contains(t, session.GetLivekitRoomName(), "voice-room-")
}

func TestVoiceGRPCVoiceRoom_memberJoinsExistingRoom(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)

	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.NoError(t, err)

	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-member"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &f.voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 2)
}

func TestVoiceGRPCVoiceRoom_nonMemberDenied(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-stranger"), f.joinReq("profile-stranger"))
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVoiceGRPCVoiceRoom_spaceMemberViewsRosterWithoutJoining(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)

	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-member"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &f.voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 1)
}

func TestVoiceGRPCVoiceRoom_leaveRemovesParticipant(t *testing.T) {
	f := startVoiceRoomFixture(t)

	_, err := f.svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), f.joinReq("profile-owner"))
	require.NoError(t, err)
	_, err = f.svc.JoinVoiceRoom(voiceTestCtx("profile-member"), f.joinReq("profile-member"))
	require.NoError(t, err)

	_, err = f.svc.LeaveVoiceRoom(voiceTestCtx("profile-member"), &callsv1.LeaveVoiceRoomRequest{
		VoiceRoomId: f.voiceRoomID,
	})
	require.NoError(t, err)

	states, err := f.svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &f.voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 1)
}

func TestVoiceGRPCVoiceRoom_max32Participants(t *testing.T) {
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	members := map[string]map[string]bool{spaceID: {"profile-owner": true}}
	for i := 1; i <= 32; i++ {
		members[spaceID][fmt.Sprintf("profile-%02d", i)] = true
	}
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	svc.SpaceMembers = &mapSpaceMembers{members: members}
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), join)
	require.NoError(t, err)
	for i := 1; i < 32; i++ {
		_, err = svc.JoinVoiceRoom(voiceTestCtx(fmt.Sprintf("profile-%02d", i)), join)
		require.NoError(t, err, "participant %d", i)
	}
	_, err = svc.JoinVoiceRoom(voiceTestCtx("profile-32"), join)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}
