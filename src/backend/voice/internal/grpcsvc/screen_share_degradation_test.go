package grpcsvc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
)

type failingRoleChecker struct{}

func (failingRoleChecker) EnsureScreenShare(context.Context, string, string, string) error {
	return errors.New("role service unavailable")
}

func TestVoiceGRPC_StartScreenShare_RoleUnavailableDeniesSpaceRoom(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	svc := &VoiceGRPC{
		Calls:  voicestore.NewMemoryCallStore(),
		Tokens: livekit.NewHS256TokenIssuer("k", "s", "ws://lk", time.Hour),
		Events: &recordingEvents{},
		Now:    func() time.Time { return now },
		Roles:  failingRoleChecker{},
	}
	store := svc.Calls.(*voicestore.MemoryCallStore)
	_, err := store.CreateCall(context.Background(), voicestore.Call{
		RoomID:             "room-role",
		LivekitRoomName:    "voice-room-role",
		VoiceRoomID:        "vr-role",
		SpaceID:            "space-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: "profile-a",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	_, err = svc.StartScreenShare(voiceTestCtx("profile-a"), &callsv1.StartScreenShareRequest{RoomId: "room-role"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}
