package store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
)

func TestCallStore_groupVoiceAddParticipantUpToLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryCallStore()

	group := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	_, err := s.CreateCall(ctx, Call{
		RoomID:             "room-1",
		LivekitRoomName:    "lk-room-1",
		ChatID:             "group-1",
		SessionKind:        group,
		InitiatorProfileID: "profile-0",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Unix(1700000000, 0).UTC(),
	})
	require.NoError(t, err)

	for i := 1; i < MaxGroupVoiceParticipants; i++ {
		_, err = s.AddParticipant(ctx, "room-1", profileID(i), MaxGroupVoiceParticipants)
		require.NoError(t, err, "participant %d", i)
	}

	call, err := s.GetCall(ctx, "room-1")
	require.NoError(t, err)
	require.Len(t, call.States, MaxGroupVoiceParticipants)

	_, err = s.AddParticipant(ctx, "room-1", "profile-overflow", MaxGroupVoiceParticipants)
	require.ErrorIs(t, err, ErrRoomFull)
}

func TestCallStore_groupVoiceJoinIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryCallStore()

	group := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE
	_, err := s.CreateCall(ctx, Call{
		RoomID:             "room-2",
		LivekitRoomName:    "lk-room-2",
		ChatID:             "group-2",
		SessionKind:        group,
		InitiatorProfileID: "profile-owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Unix(1700000000, 0).UTC(),
	})
	require.NoError(t, err)

	_, err = s.AddParticipant(ctx, "room-2", "profile-member", MaxGroupVoiceParticipants)
	require.NoError(t, err)

	call, err := s.AddParticipant(ctx, "room-2", "profile-member", MaxGroupVoiceParticipants)
	require.NoError(t, err)
	require.Len(t, call.States, 2)
}

// TestCallStore_oneActiveVoicePerProfileUntilLeave characterizes the documented
// Voice Service invariant: a profile, rather than a device connection, can
// occupy only one active voice session.  A successful leave frees that profile
// for another open voice session.
func TestCallStore_oneActiveVoicePerProfileUntilLeave(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryCallStore()
	voiceRoom := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM

	_, err := s.CreateCall(ctx, Call{
		RoomID:             "room-first",
		LivekitRoomName:    "lk-first",
		VoiceRoomID:        "voice-room-first",
		SessionKind:        voiceRoom,
		InitiatorProfileID: "profile-shared",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)

	_, err = s.CreateCall(ctx, Call{
		RoomID:             "room-second",
		LivekitRoomName:    "lk-second",
		VoiceRoomID:        "voice-room-second",
		SessionKind:        voiceRoom,
		InitiatorProfileID: "profile-shared",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.ErrorIs(t, err, ErrActiveCall)

	_, err = s.RemoveParticipant(ctx, "room-first", "profile-shared")
	require.NoError(t, err)

	_, err = s.CreateCall(ctx, Call{
		RoomID:             "room-second",
		LivekitRoomName:    "lk-second",
		VoiceRoomID:        "voice-room-second",
		SessionKind:        voiceRoom,
		InitiatorProfileID: "profile-shared",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)
}

func TestCallStore_spaceAdmissionHasOneActiveRoomIncarnation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryCallStore()
	voiceRoom := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM
	for _, roomID := range []string{"pending-first", "pending-second"} {
		_, err := s.CreateCall(ctx, Call{
			RoomID: roomID, LivekitRoomName: "lk-" + roomID, VoiceRoomID: "voice-room-single",
			SpaceID: "space", SessionKind: voiceRoom, InitiatorProfileID: roomID,
			MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
			Status:    callsv1.CallStatus_CALL_STATUS_UNSPECIFIED,
		})
		require.NoError(t, err)
	}
	participant := func(profileID string) SpaceMediaParticipant {
		return SpaceMediaParticipant{AccountID: profileID, AdmissionOperationID: profileID, ProfileID: profileID,
			Identity: "identity-" + profileID, Generation: "generation-" + profileID,
			Issued: SpaceMediaGrant{SessionEpoch: 1, AccessEpoch: 1, PolicyEpoch: 1, CanJoin: true, CanSubscribe: true}}
	}
	_, err := s.AdmitSpaceMediaParticipant(ctx, "pending-first", participant("account-a"), MaxVoiceRoomParticipants)
	require.NoError(t, err)
	_, err = s.AdmitSpaceMediaParticipant(ctx, "pending-second", participant("account-b"), MaxVoiceRoomParticipants)
	require.ErrorIs(t, err, ErrActiveCall, "only one committed call projection may own a voice-room ID")
	active, err := s.GetCallByVoiceRoomID(ctx, "voice-room-single")
	require.NoError(t, err)
	require.Equal(t, "pending-first", active.RoomID)
	loser, err := s.GetCall(ctx, "pending-second")
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_UNSPECIFIED, loser.Status, "losing shell remains unpublished for exact recovery cleanup")
}

func TestCallStore_GetActiveGroupCallForChat(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryCallStore()
	group := callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE

	_, err := s.CreateCall(ctx, Call{
		RoomID:             "room-gv",
		ChatID:             "group-chat-1",
		SessionKind:        group,
		InitiatorProfileID: "profile-owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
	})
	require.NoError(t, err)

	call, err := s.GetActiveGroupCallForChat(ctx, "group-chat-1")
	require.NoError(t, err)
	require.Equal(t, "room-gv", call.RoomID)

	_, err = s.GetActiveGroupCallForChat(ctx, "other-group")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestCall_IsGroupVoiceAndParticipant(t *testing.T) {
	t.Parallel()
	group := Call{
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: "owner",
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		States: map[string]ParticipantState{
			"owner":  {ProfileID: "owner"},
			"member": {ProfileID: "member"},
		},
	}
	require.True(t, group.IsGroupVoice())
	require.True(t, group.IsParticipant("member"))
	require.False(t, group.IsParticipant("stranger"))
	require.ElementsMatch(t, []string{"owner", "member"}, group.ProfileIDs())

	dm := Call{
		InitiatorProfileID: "a",
		CalleeProfileID:    "b",
		Status:             callsv1.CallStatus_CALL_STATUS_RINGING,
	}
	require.False(t, dm.IsGroupVoice())
	require.True(t, dm.IsParticipant("b"))
}

func profileID(i int) string {
	return fmt.Sprintf("profile-%02d", i)
}
