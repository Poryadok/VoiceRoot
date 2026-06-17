package grpcsvc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"voice/backend/voice/internal/livekit"
	voicestore "voice/backend/voice/internal/store"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
	eventsv1 "voice.app/voice/events/v1"
)

type recordingEvents struct {
	incoming []*eventsv1.CallIncoming
	accepted []*eventsv1.CallAccepted
	declined []*eventsv1.CallDeclined
	missed   []*eventsv1.CallMissed
	ended    []*eventsv1.CallEnded
	states   []*eventsv1.VoiceStateChanged
	started  []*eventsv1.ScreenShareStarted
	stopped  []*eventsv1.ScreenShareStopped
}

func (r *recordingEvents) PublishCallIncoming(_ context.Context, ev *eventsv1.CallIncoming) error {
	r.incoming = append(r.incoming, ev)
	return nil
}

func (r *recordingEvents) PublishCallAccepted(_ context.Context, ev *eventsv1.CallAccepted) error {
	r.accepted = append(r.accepted, ev)
	return nil
}

func (r *recordingEvents) PublishCallDeclined(_ context.Context, ev *eventsv1.CallDeclined) error {
	r.declined = append(r.declined, ev)
	return nil
}

func (r *recordingEvents) PublishCallMissed(_ context.Context, ev *eventsv1.CallMissed) error {
	r.missed = append(r.missed, ev)
	return nil
}

func (r *recordingEvents) PublishCallEnded(_ context.Context, ev *eventsv1.CallEnded) error {
	r.ended = append(r.ended, ev)
	return nil
}

func (r *recordingEvents) PublishVoiceStateChanged(_ context.Context, ev *eventsv1.VoiceStateChanged) error {
	r.states = append(r.states, ev)
	return nil
}

func (r *recordingEvents) PublishScreenShareStarted(_ context.Context, ev *eventsv1.ScreenShareStarted) error {
	r.started = append(r.started, ev)
	return nil
}

func (r *recordingEvents) PublishScreenShareStopped(_ context.Context, ev *eventsv1.ScreenShareStopped) error {
	r.stopped = append(r.stopped, ev)
	return nil
}

func voiceTestCtx(profileID string) context.Context {
	return metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-voice-profile-id", profileID))
}

func strPtr(v string) *string {
	return &v
}

func mediaPtr(v callsv1.CallMediaKind) *callsv1.CallMediaKind {
	return &v
}

func newTestVoiceService(now time.Time, events *recordingEvents) *VoiceGRPC {
	return &VoiceGRPC{
		Calls:       voicestore.NewMemoryCallStore(),
		Tokens:      livekit.NewHS256TokenIssuer("dev-key", "dev-secret", "ws://livekit:7880", time.Hour),
		Events:      events,
		Now:         func() time.Time { return now },
		RingTimeout: 30 * time.Second,
	}
}

func newTestGroupVoiceService(now time.Time, events *recordingEvents) *VoiceGRPC {
	members := map[string]map[string]bool{
		"group-chat-1": {
			"profile-owner":  true,
			"profile-member": true,
		},
	}
	for i := 1; i <= 33; i++ {
		members["group-chat-1"][fmt.Sprintf("profile-%02d", i)] = true
	}
	svc := newTestVoiceService(now, events)
	svc.ChatMembers = &mapChatMembers{members: members}
	return svc
}

func TestVoiceGRPCStartAcceptTokenStateAndEnd(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)

	start, err := svc.StartCall(voiceTestCtx("profile-a"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-1"},
		CalleeProfileId: strPtr("profile-b"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO),
	})
	require.NoError(t, err)
	call := start.GetCallSession()
	require.NotEmpty(t, call.GetRoomId())
	require.Equal(t, "chat-1", call.GetLinkedChat().GetId())
	require.Equal(t, "profile-a", call.GetInitiatorProfileId())
	require.Equal(t, "profile-b", call.GetCalleeProfileId())
	require.Equal(t, callsv1.CallMediaKind_CALL_MEDIA_KIND_VIDEO, call.GetMediaKind())
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_RINGING, call.GetStatus())
	require.Len(t, events.incoming, 1)
	require.Equal(t, "profile-b", events.incoming[0].GetCalleeProfileId())

	_, err = svc.StartCall(voiceTestCtx("profile-a"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-2"},
		CalleeProfileId: strPtr("profile-c"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	accepted, err := svc.AcceptCall(voiceTestCtx("profile-b"), &callsv1.AcceptCallRequest{RoomId: call.GetRoomId()})
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, accepted.GetCallSession().GetStatus())
	require.Len(t, events.accepted, 1)

	token, err := svc.GetJoinToken(voiceTestCtx("profile-a"), &callsv1.GetJoinTokenRequest{RoomId: call.GetRoomId()})
	require.NoError(t, err)
	require.NotEmpty(t, token.GetJwt())
	require.Equal(t, "ws://livekit:7880", token.GetLivekitUrl())

	muted := true
	video := false
	_, err = svc.UpdateVoiceState(voiceTestCtx("profile-a"), &callsv1.UpdateVoiceStateRequest{
		RoomId:    call.GetRoomId(),
		IsMuted:   &muted,
		IsVideoOn: &video,
	})
	require.NoError(t, err)
	require.Len(t, events.states, 1)

	states, err := svc.GetVoiceStates(voiceTestCtx("profile-a"), &callsv1.GetVoiceStatesRequest{RoomId: call.GetRoomId()})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 2)

	_, err = svc.EndCall(voiceTestCtx("profile-b"), &callsv1.EndCallRequest{RoomId: call.GetRoomId()})
	require.NoError(t, err)
	require.Len(t, events.ended, 1)
}

func TestVoiceGRPCDeclineAndMissedTimeout(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)

	start, err := svc.StartCall(voiceTestCtx("caller"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-1"},
		CalleeProfileId: strPtr("callee"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)

	_, err = svc.DeclineCall(voiceTestCtx("callee"), &callsv1.DeclineCallRequest{RoomId: start.GetCallSession().GetRoomId()})
	require.NoError(t, err)
	require.Len(t, events.declined, 1)
	require.Len(t, events.ended, 1)

	start, err = svc.StartCall(voiceTestCtx("caller"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-2"},
		CalleeProfileId: strPtr("callee"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)

	svc.Now = func() time.Time { return now.Add(31 * time.Second) }
	missed, err := svc.MarkExpiredCallsMissed(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, missed)
	require.Len(t, events.missed, 1)
	require.Equal(t, start.GetCallSession().GetRoomId(), events.missed[0].GetRoomId())
}

func TestVoiceGRPCRejectsInvalidCallerAndParticipants(t *testing.T) {
	svc := newTestVoiceService(time.Now(), &recordingEvents{})

	_, err := svc.StartCall(context.Background(), &callsv1.StartCallRequest{})
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	_, err = svc.StartCall(voiceTestCtx("profile-a"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-1"},
		CalleeProfileId: strPtr("profile-a"),
	})
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	start, err := svc.StartCall(voiceTestCtx("profile-a"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-1"},
		CalleeProfileId: strPtr("profile-b"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)

	_, err = svc.AcceptCall(voiceTestCtx("profile-c"), &callsv1.AcceptCallRequest{RoomId: start.GetCallSession().GetRoomId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestVoiceGRPCJoinLeaveAndGetActiveCall(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)

	start, err := svc.StartCall(voiceTestCtx("profile-a"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-1"},
		CalleeProfileId: strPtr("profile-b"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)
	roomID := start.GetCallSession().GetRoomId()

	_, err = svc.GetActiveCall(voiceTestCtx("profile-a"), &callsv1.GetActiveCallRequest{})
	require.NoError(t, err)

	_, err = svc.AcceptCall(voiceTestCtx("profile-b"), &callsv1.AcceptCallRequest{RoomId: roomID})
	require.NoError(t, err)

	joined, err := svc.JoinCall(voiceTestCtx("profile-a"), &callsv1.JoinCallRequest{RoomId: roomID})
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, joined.GetCallSession().GetStatus())

	active, err := svc.GetActiveCall(voiceTestCtx("profile-a"), &callsv1.GetActiveCallRequest{})
	require.NoError(t, err)
	require.Equal(t, roomID, active.GetCallSession().GetRoomId())

	_, err = svc.LeaveCall(voiceTestCtx("profile-b"), &callsv1.LeaveCallRequest{RoomId: roomID})
	require.NoError(t, err)
	require.Len(t, events.ended, 1)

	_, err = svc.GetActiveCall(voiceTestCtx("profile-a"), &callsv1.GetActiveCallRequest{})
	require.Equal(t, codes.NotFound, status.Code(err))
}

func guestVoiceTestCtx(profileID string) context.Context {
	md := metadata.Pairs(
		"x-voice-profile-id", profileID,
		"x-voice-account-type", "guest",
	)
	return metadata.NewIncomingContext(context.Background(), md)
}

// TestVoiceGRPC_GuestStartCall_PermissionDenied documents auth-and-contacts.md: guests cannot initiate calls.
func TestVoiceGRPC_GuestStartCall_PermissionDenied(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	events := &recordingEvents{}
	svc := newTestVoiceService(now, events)

	_, err := svc.StartCall(guestVoiceTestCtx("profile-a"), &callsv1.StartCallRequest{
		LinkedChat:      &chatv1.ChatRef{Id: "chat-1"},
		CalleeProfileId: strPtr("profile-b"),
		MediaKind:       mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
	require.Empty(t, events.incoming)
}
