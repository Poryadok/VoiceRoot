package grpcsvc

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
)

func TestVoiceGRPC_StartStopScreenShare(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)
	store := svc.Calls.(*voicestore.MemoryCallStore)
	ctx := voiceTestCtx("profile-a")

	_, err := store.CreateCall(ctx, voicestore.Call{
		RoomID:             "room-ss",
		LivekitRoomName:    "voice-dm-room-ss",
		ChatID:             "chat-1",
		InitiatorProfileID: "profile-a",
		CalleeProfileID:    "profile-b",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	startResp, err := svc.StartScreenShare(ctx, &callsv1.StartScreenShareRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	require.NotEmpty(t, startResp.GetScreenShareSession().GetStreamId())
	require.Len(t, events.started, 1)
	require.Equal(t, "room-ss", events.started[0].GetRoomId())
	require.Equal(t, "profile-a", events.started[0].GetProfileId())

	states, err := svc.GetVoiceStates(ctx, &callsv1.GetVoiceStatesRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	require.True(t, states.GetParticipants()[0].GetIsScreenSharing())

	_, err = svc.StopScreenShare(ctx, &callsv1.StopScreenShareRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	require.Len(t, events.stopped, 1)

	states, err = svc.GetVoiceStates(ctx, &callsv1.GetVoiceStatesRequest{RoomId: "room-ss"})
	require.NoError(t, err)
	for _, p := range states.GetParticipants() {
		if p.GetProfileId() == "profile-a" {
			require.False(t, p.GetIsScreenSharing())
		}
	}
}

func TestVoiceGRPC_StartScreenShare_LimitAndPermission(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)
	store := svc.Calls.(*voicestore.MemoryCallStore)
	ctx := voiceTestCtx("profile-owner")

	_, err := store.CreateCall(ctx, voicestore.Call{
		RoomID:             "room-limit",
		LivekitRoomName:    "voice-group-limit",
		ChatID:             "group-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE,
		InitiatorProfileID: "profile-owner",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
		States: map[string]voicestore.ParticipantState{
			"profile-owner": {ProfileID: "profile-owner"},
			"profile-2":     {ProfileID: "profile-2"},
			"profile-3":     {ProfileID: "profile-3"},
			"profile-4":     {ProfileID: "profile-4"},
		},
	})
	require.NoError(t, err)

	for _, id := range []string{"profile-owner", "profile-2", "profile-3"} {
		pctx := voiceTestCtx(id)
		_, err := svc.StartScreenShare(pctx, &callsv1.StartScreenShareRequest{RoomId: "room-limit"})
		require.NoError(t, err)
	}

	_, err = svc.StartScreenShare(voiceTestCtx("profile-4"), &callsv1.StartScreenShareRequest{RoomId: "room-limit"})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))

	_, err = svc.StartScreenShare(voiceTestCtx("outsider"), &callsv1.StartScreenShareRequest{RoomId: "room-limit"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVoiceGRPC_StartScreenShare_VoiceRoomRoleCheck(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := &VoiceGRPC{
		Calls:  voicestore.NewMemoryCallStore(),
		Tokens: livekit.NewHS256TokenIssuer("dev-key", "dev-secret", "ws://livekit:7880", time.Hour),
		Events: events,
		Now:    func() time.Time { return now },
		Roles: &mapRolePermissions{allowed: map[string]map[string]bool{
			"space-1": {"profile-allowed": true},
		}},
	}
	store := svc.Calls.(*voicestore.MemoryCallStore)
	_, err := store.CreateCall(context.Background(), voicestore.Call{
		RoomID:             "room-vr",
		LivekitRoomName:    "voice-room-vr",
		VoiceRoomID:        "vr-1",
		SpaceID:            "space-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: "profile-allowed",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	_, err = svc.StartScreenShare(voiceTestCtx("profile-allowed"), &callsv1.StartScreenShareRequest{RoomId: "room-vr"})
	require.NoError(t, err)

	_, err = store.AddParticipant(context.Background(), "room-vr", "profile-denied", voicestore.MaxVoiceRoomParticipants)
	require.NoError(t, err)
	_, err = svc.StartScreenShare(voiceTestCtx("profile-denied"), &callsv1.StartScreenShareRequest{RoomId: "room-vr"})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVoiceGRPC_LeaveVoiceRoom_ClearsScreenShare(t *testing.T) {
	t.Parallel()
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := &VoiceGRPC{
		Calls:  voicestore.NewMemoryCallStore(),
		Tokens: livekit.NewHS256TokenIssuer("k", "s", "ws://lk", time.Hour),
		Events: events,
		Now:    func() time.Time { return now },
		Roles:  &mapRolePermissions{allowed: map[string]map[string]bool{"space-1": {"profile-a": true}}},
	}
	store := svc.Calls.(*voicestore.MemoryCallStore)
	_, err := store.CreateCall(context.Background(), voicestore.Call{
		RoomID:             "room-leave",
		LivekitRoomName:    "voice-room-leave",
		VoiceRoomID:        "vr-leave",
		SpaceID:            "space-1",
		SessionKind:        callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM,
		InitiatorProfileID: "profile-a",
		MediaKind:          callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO,
		Status:             callsv1.CallStatus_CALL_STATUS_ACTIVE,
		StartedAt:          now,
	})
	require.NoError(t, err)

	_, err = svc.StartScreenShare(voiceTestCtx("profile-a"), &callsv1.StartScreenShareRequest{RoomId: "room-leave"})
	require.NoError(t, err)

	_, err = svc.LeaveVoiceRoom(voiceTestCtx("profile-a"), &callsv1.LeaveVoiceRoomRequest{VoiceRoomId: "vr-leave"})
	require.NoError(t, err)
	require.Len(t, events.stopped, 1)
}
