package store

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	callsv1 "voice.app/voice/calls/v1"
)

func TestCallStore_ScreenShareLifecycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryCallStore()
	_, err := s.CreateCall(ctx, Call{
		RoomID:             "room-ss",
		LivekitRoomName:    "lk-ss",
		ChatID:             "chat-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: "p1",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Unix(1700000000, 0).UTC(),
	})
	require.NoError(t, err)
	for _, id := range []string{"p2", "p3", "p4"} {
		_, err = s.AddParticipant(ctx, "room-ss", id, MaxGroupVoiceParticipants)
		require.NoError(t, err)
	}

	_, entry, err := s.StartScreenShare(ctx, "room-ss", "p1", "stream-1")
	require.NoError(t, err)
	require.Equal(t, "stream-1", entry.StreamID)

	call, err := s.GetCall(ctx, "room-ss")
	require.NoError(t, err)
	require.True(t, call.States["p1"].IsScreenSharing)
	require.Len(t, call.ScreenShares, 1)

	_, entry, err = s.StartScreenShare(ctx, "room-ss", "p1", "stream-1b")
	require.NoError(t, err)
	require.Equal(t, "stream-1b", entry.StreamID)
	call, err = s.GetCall(ctx, "room-ss")
	require.NoError(t, err)
	require.Len(t, call.ScreenShares, 1)

	for _, id := range []string{"p2", "p3"} {
		_, _, err = s.StartScreenShare(ctx, "room-ss", id, "stream-"+id)
		require.NoError(t, err)
	}
	_, _, err = s.StartScreenShare(ctx, "room-ss", "p4", "stream-p4")
	require.ErrorIs(t, err, ErrScreenShareLimit)

	_, err = s.StopScreenShare(ctx, "room-ss", "p1", "")
	require.NoError(t, err)
	call, err = s.GetCall(ctx, "room-ss")
	require.NoError(t, err)
	require.False(t, call.States["p1"].IsScreenSharing)
}

func TestCallStore_StopScreenSharesForProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := NewMemoryCallStore()
	_, err := s.CreateCall(ctx, Call{
		RoomID:             "room-clear",
		InitiatorProfileID: "p1",
		CalleeProfileID:    "p2",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          time.Unix(1700000000, 0).UTC(),
	})
	require.NoError(t, err)
	_, _, err = s.StartScreenShare(ctx, "room-clear", "p1", "s1")
	require.NoError(t, err)
	_, err = s.StopScreenSharesForProfile(ctx, "room-clear", "p1")
	require.NoError(t, err)
	call, err := s.GetCall(ctx, "room-clear")
	require.NoError(t, err)
	require.Empty(t, call.ScreenShares)
	require.False(t, call.States["p1"].IsScreenSharing)
}
