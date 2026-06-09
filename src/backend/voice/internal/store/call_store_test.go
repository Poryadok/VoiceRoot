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
