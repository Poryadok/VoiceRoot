package grpcsvc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
)

// TestJoinVoiceRoom_SpaceMembersNotConfigured documents OPERATIONS.md fail-closed when SPACE_GRPC_ADDR is unwired.
func TestJoinVoiceRoom_SpaceMembersNotConfigured(t *testing.T) {
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()

	_, err := svc.JoinVoiceRoom(voiceTestCtx("profile-owner"), &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestLeaveVoiceRoom_SpaceMembersNotConfigured documents fail-closed leave when membership S2S is unwired.
func TestLeaveVoiceRoom_SpaceMembersNotConfigured(t *testing.T) {
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	profileID := "profile-owner"
	now := time.Unix(1700000000, 0).UTC()

	_, err := svc.Calls.CreateCall(t.Context(), voicestore.Call{
		RoomID:             uuid.New().String(),
		LivekitRoomName:    "voice-room-" + voiceRoomID,
		VoiceRoomID:        voiceRoomID,
		SpaceID:            spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: profileID,
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	_, err = svc.LeaveVoiceRoom(voiceTestCtx(profileID), &callsv1.LeaveVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestGetVoiceStates_SpaceMembersNotConfigured documents fail-closed voice state read for space rooms.
func TestGetVoiceStates_SpaceMembersNotConfigured(t *testing.T) {
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	voiceRoomID := uuid.New().String()

	_, err := svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &voiceRoomID,
	})
	require.NoError(t, err)

	spaceID := uuid.New().String()
	now := time.Unix(1700000000, 0).UTC()
	_, err = svc.Calls.CreateCall(t.Context(), voicestore.Call{
		RoomID:             uuid.New().String(),
		LivekitRoomName:    "voice-room-" + voiceRoomID,
		VoiceRoomID:        voiceRoomID,
		SpaceID:            spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: "profile-owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	_, err = svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &voiceRoomID,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
