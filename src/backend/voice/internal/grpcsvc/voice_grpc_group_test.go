package grpcsvc

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	callsv1 "voice.app/voice/calls/v1"
	chatv1 "voice.app/voice/chat/v1"
)

// TestVoiceGRPCStartGroupVoice_createsActiveSession documents PLAN Phase 4 / voice-chat.md:
// group voice in a text group starts an active temporary room (no DM callee / ringing).
func TestVoiceGRPCStartGroupVoice_createsActiveSession(t *testing.T) {
	events := &recordingEvents{}
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), events)
	group := chatv1.ChatType_CHAT_TYPE_GROUP

	start, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)

	call := start.GetCallSession()
	require.Equal(t, callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, call.GetRoomTypeEnum())
	require.Equal(t, "group_voice", call.GetRoomType())
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, call.GetStatus())
	require.Equal(t, "group-chat-1", call.GetLinkedChat().GetId())
	require.Equal(t, "profile-owner", call.GetInitiatorProfileId())
	require.Empty(t, call.GetCalleeProfileId())
}

// TestVoiceGRPCGroupVoice_memberJoinsActiveCall documents PLAN Phase 4: members join an active group call.
func TestVoiceGRPCGroupVoice_memberJoinsActiveCall(t *testing.T) {
	events := &recordingEvents{}
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), events)
	group := chatv1.ChatType_CHAT_TYPE_GROUP

	start, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)
	roomID := start.GetCallSession().GetRoomId()

	joined, err := svc.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: roomID})
	require.NoError(t, err)
	require.Equal(t, callsv1.CallStatus_CALL_STATUS_ACTIVE, joined.GetCallSession().GetStatus())

	states, err := svc.GetVoiceStates(voiceTestCtx("profile-owner"), &callsv1.GetVoiceStatesRequest{RoomId: roomID})
	require.NoError(t, err)
	require.Len(t, states.GetParticipants(), 2)
}

// TestVoiceGRPCGroupVoice_nonMemberDenied ensures only group members can join.
func TestVoiceGRPCGroupVoice_nonMemberDenied(t *testing.T) {
	events := &recordingEvents{}
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), events)
	group := chatv1.ChatType_CHAT_TYPE_GROUP

	start, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)

	_, err = svc.JoinCall(voiceTestCtx("profile-stranger"), &callsv1.JoinCallRequest{RoomId: start.GetCallSession().GetRoomId()})
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestVoiceGRPCGroupVoice_max32Participants documents voice-chat.md free tier limit (32).
func TestVoiceGRPCGroupVoice_max32Participants(t *testing.T) {
	events := &recordingEvents{}
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), events)
	group := chatv1.ChatType_CHAT_TYPE_GROUP

	start, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)
	roomID := start.GetCallSession().GetRoomId()

	for i := 1; i < 32; i++ {
		_, err = svc.JoinCall(voiceTestCtx(fmt.Sprintf("profile-%02d", i)), &callsv1.JoinCallRequest{RoomId: roomID})
		require.NoError(t, err, "participant %d should join", i)
	}

	_, err = svc.JoinCall(voiceTestCtx("profile-33"), &callsv1.JoinCallRequest{RoomId: roomID})
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestVoiceGRPCGroupVoice_joinTokenAfterJoin documents LiveKit token issuance for group participants.
func TestVoiceGRPCGroupVoice_joinTokenAfterJoin(t *testing.T) {
	events := &recordingEvents{}
	svc := newTestGroupVoiceService(time.Unix(1700000000, 0).UTC(), events)
	group := chatv1.ChatType_CHAT_TYPE_GROUP

	start, err := svc.StartCall(voiceTestCtx("profile-owner"), &callsv1.StartCallRequest{
		RoomTypeEnum: callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE.Enum(),
		LinkedChat:   &chatv1.ChatRef{Id: "group-chat-1", Type: &group},
		MediaKind:    mediaPtr(callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO),
	})
	require.NoError(t, err)
	roomID := start.GetCallSession().GetRoomId()

	_, err = svc.JoinCall(voiceTestCtx("profile-member"), &callsv1.JoinCallRequest{RoomId: roomID})
	require.NoError(t, err)

	token, err := svc.GetJoinToken(voiceTestCtx("profile-member"), &callsv1.GetJoinTokenRequest{RoomId: roomID})
	require.NoError(t, err)
	require.NotEmpty(t, token.GetJwt())
	require.Equal(t, "ws://livekit:7880", token.GetLivekitUrl())
}
