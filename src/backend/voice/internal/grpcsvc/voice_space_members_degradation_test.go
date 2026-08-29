package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

// TestJoinVoiceRoom_SpaceMembersNotConfigured_failOpen documents PLAN.md: Voice fail-open
// when SpaceMembers S2S is unwired (SPACE_GRPC_ADDR unset in main.go).
func TestJoinVoiceRoom_SpaceMembersNotConfigured_failOpen(t *testing.T) {
	t.Parallel()
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-stranger"), join)
	require.NoError(t, err)
}

// TestLeaveVoiceRoom_SpaceMembersNotConfigured_failOpen documents fail-open leave when
// SpaceMembers S2S is unwired.
func TestLeaveVoiceRoom_SpaceMembersNotConfigured_failOpen(t *testing.T) {
	t.Parallel()
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-stranger"), join)
	require.NoError(t, err)

	_, err = svc.LeaveVoiceRoom(voiceTestCtx("profile-stranger"), &callsv1.LeaveVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
	})
	require.NoError(t, err)
}

// TestGetVoiceStates_SpaceMembersNotConfigured_failOpen documents fail-open roster read
// when SpaceMembers S2S is unwired.
func TestGetVoiceStates_SpaceMembersNotConfigured_failOpen(t *testing.T) {
	t.Parallel()
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	join := &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	}

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), join)
	require.NoError(t, err)

	states, err := svc.GetVoiceStates(voiceTestCtx("profile-stranger"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &voiceRoomID,
	})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 1)
}

func TestEnsureSpaceMember_nilClientNoOp(t *testing.T) {
	t.Parallel()
	svc := &VoiceGRPC{}
	require.NoError(t, svc.ensureSpaceMember(t.Context(), uuid.New().String(), "profile-a"))
}
