package grpcsvc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	callsv1 "voice.app/voice/calls/v1"
	spacev1 "voice.app/voice/space/v1"
	voicestore "voice/backend/voice/internal/store"
)

func TestVoiceSessionBinding_ProjectsOnlyCompletePersistedRoomTuple(t *testing.T) {
	room, space := uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name                       string
		kind                       callsv1.VoiceSessionKind
		voiceRoom, space, roomType string
		bound                      bool
	}{
		{"room", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, room, space, "voice_room", true},
		{"preserves UUID spelling", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, "AAAAAAAA-AAAA-4AAA-8AAA-AAAAAAAAAAAA", "BBBBBBBB-BBBB-4BBB-8BBB-BBBBBBBBBBBB", "voice_room", true},
		{"legacy missing space", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, room, "", "voice_room", false},
		{"missing room", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, "", space, "voice_room", false},
		{"malformed space", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, room, "bad", "voice_room", false},
		{"malformed room", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_VOICE_ROOM, "bad", space, "voice_room", false},
		{"direct incidental tuple", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_CALL, room, space, "call", false},
		{"group incidental tuple", callsv1.VoiceSessionKind_VOICE_SESSION_KIND_GROUP_VOICE, room, space, "group_voice", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, member := uuid.NewString(), uuid.NewString()
			call := voicestore.Call{RoomID: uuid.NewString(), LivekitRoomName: "lk-binding", ChatID: uuid.NewString(), VoiceRoomID: tc.voiceRoom, SpaceID: tc.space, SessionKind: tc.kind, InitiatorProfileID: owner, CalleeProfileID: member, MediaKind: callsv1.CallMediaKind_CALL_MEDIA_KIND_AUDIO, Status: callsv1.CallStatus_CALL_STATUS_ACTIVE, States: map[string]voicestore.ParticipantState{owner: {ProfileID: owner}, member: {ProfileID: member}}}
			active, joined := callToProto(call), voiceSessionToProto(call)
			require.Equal(t, call.RoomID, active.GetRoomId())
			require.Equal(t, call.LivekitRoomName, joined.GetLivekitRoomName())
			require.Equal(t, tc.roomType, active.GetRoomType())
			// Existing room fields are preserved even in legacy/incomplete records.
			require.Equal(t, tc.voiceRoom, active.GetVoiceRoomId())
			require.Equal(t, tc.voiceRoom, joined.GetVoiceRoomId())
			events := &recordingEvents{}
			svc := newTestVoiceService(time.Now(), events)
			svc.publishCallStarted(t.Context(), call)
			require.Len(t, events.startedCall, 1)
			event := events.startedCall[0]
			require.ElementsMatch(t, []string{owner, member}, event.GetProfileIds())
			require.Equal(t, call.RoomID, event.GetRoomId())
			require.Equal(t, call.ChatID, event.GetChatId())
			require.Equal(t, "audio", event.GetMediaKind())
			require.Equal(t, call.LivekitRoomName, event.GetLivekitRoomName())
			require.Equal(t, tc.roomType, event.GetRoomType())
			if tc.bound {
				require.Equal(t, tc.space, active.GetSpaceId())
				require.Equal(t, tc.space, joined.GetSpaceId())
				require.Equal(t, tc.voiceRoom, event.GetVoiceRoomId())
				require.Equal(t, tc.space, event.GetSpaceId())
			} else {
				require.Nil(t, active.SpaceId)
				require.Nil(t, joined.SpaceId)
				require.Nil(t, event.SpaceId)
				require.Nil(t, event.VoiceRoomId)
			}
			raw, err := (protojson.MarshalOptions{UseProtoNames: true}).Marshal(active)
			require.NoError(t, err)
			var payload map[string]any
			require.NoError(t, json.Unmarshal(raw, &payload))
			if tc.bound {
				require.Equal(t, tc.space, payload["space_id"])
			} else {
				require.NotContains(t, payload, "space_id")
			}
		})
	}
}

func TestVoiceSessionBinding_CanonicalJoinAndActiveResponse(t *testing.T) {
	room, space, profile := uuid.NewString(), uuid.NewString(), uuid.NewString()
	events := &recordingEvents{}
	svc := newTestVoiceService(time.Now(), events)
	resolver := &canonicalAccessResolver{result: CanonicalVoiceRoomAccess{SpaceID: space, Member: true, Active: true}}
	svc.VoiceRoomAccessResolver = resolver
	joined, err := svc.JoinVoiceRoom(voiceTestCtx(profile), &callsv1.JoinVoiceRoomRequest{VoiceRoomId: room, Space: &spacev1.SpaceRef{Id: space}})
	require.NoError(t, err)
	require.Equal(t, space, joined.GetVoiceSession().GetSpaceId())
	require.Equal(t, room, joined.GetVoiceSession().GetVoiceRoomId())
	stored, err := svc.Calls.GetCall(t.Context(), joined.GetVoiceSession().GetRoomId())
	require.NoError(t, err)
	require.Equal(t, space, stored.SpaceID)
	require.Equal(t, room, stored.VoiceRoomID)
	resolverCalls := len(resolver.calls)
	require.Positive(t, resolverCalls)
	active, err := svc.GetActiveCall(voiceTestCtx(profile), &callsv1.GetActiveCallRequest{})
	require.NoError(t, err)
	require.Len(t, resolver.calls, resolverCalls, "active locator projection must not add a resolver dependency")
	require.Equal(t, space, active.GetCallSession().GetSpaceId())
	require.Equal(t, room, active.GetCallSession().GetVoiceRoomId())
	require.Equal(t, joined.GetVoiceSession().GetRoomId(), active.GetCallSession().GetRoomId())
	require.Len(t, events.startedCall, 1)
	require.Equal(t, space, events.startedCall[0].GetSpaceId())
	require.Equal(t, room, events.startedCall[0].GetVoiceRoomId())
	require.Equal(t, []string{profile}, events.startedCall[0].GetProfileIds())
}
