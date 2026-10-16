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

// TestJoinVoiceRoom_CanonicalResolverNotConfiguredFailsClosed documents the authoritative Space lookup boundary.
func TestJoinVoiceRoom_CanonicalResolverNotConfiguredFailsClosed(t *testing.T) {
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	configureReadySpaceMediaFixture(svc, nil)
	svc.Roles = &canonicalRolePermissions{}
	profileID := uuid.New().String()

	_, err := joinSpaceVoiceUser(t, svc, profileID, &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
		Space:       &spacev1.SpaceRef{Id: spaceID},
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestLeaveVoiceRoom_UnjournaledSpaceParticipantFailsClosed documents that a Space roster cannot use legacy leave mutation.
func TestLeaveVoiceRoom_UnjournaledSpaceParticipantFailsClosed(t *testing.T) {
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	spaceID := uuid.New().String()
	voiceRoomID := uuid.New().String()
	profileID := uuid.New().String()
	now := time.Unix(1700000000, 0).UTC()
	configureReadySpaceMediaFixture(svc, &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{
		SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 11,
	}})
	svc.Roles = &canonicalRolePermissions{}

	call, err := svc.Calls.CreateCall(t.Context(), voicestore.Call{
		RoomID:             uuid.New().String(),
		LivekitRoomName:    "voice-room-" + voiceRoomID,
		VoiceRoomID:        voiceRoomID,
		SpaceID:            spaceID,
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: profileID,
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
		States:             map[string]voicestore.ParticipantState{profileID: {ProfileID: profileID}},
	})
	require.NoError(t, err)

	_, err = svc.LeaveVoiceRoom(voiceTestCtx(profileID), &callsv1.LeaveVoiceRoomRequest{
		VoiceRoomId: voiceRoomID,
	})
	require.Error(t, err)
	require.Equal(t, codes.Unavailable, status.Code(err))
	current, err := svc.Calls.GetCall(t.Context(), call.RoomID)
	require.NoError(t, err)
	require.True(t, current.IsParticipant(profileID), "an unjournaled Space projection cannot use legacy leave mutation")
}

// TestGetVoiceStates_SpaceMembersNotConfigured documents the legacy membership-read dependency for Space roster reads.
func TestGetVoiceStates_SpaceMembersNotConfigured(t *testing.T) {
	svc := newTestVoiceService(time.Unix(1700000000, 0).UTC(), &recordingEvents{})
	voiceRoomID := uuid.New().String()
	spaceID := uuid.New().String()
	profileID := uuid.New().String()
	configureReadySpaceMediaFixture(svc, &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{
		SpaceID: spaceID, Member: true, Active: true, AccessEpoch: 11,
	}})
	svc.Roles = &canonicalRolePermissions{}
	_, err := joinSpaceVoiceUser(t, svc, profileID, &callsv1.JoinVoiceRoomRequest{
		VoiceRoomId: voiceRoomID, Space: &spacev1.SpaceRef{Id: spaceID},
	})
	require.NoError(t, err, "the Space Join must succeed before exercising the legacy roster dependency")

	_, err = svc.GetVoiceStates(voiceTestCtx(profileID), &callsv1.GetVoiceStatesRequest{
		VoiceRoomId: &voiceRoomID,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
