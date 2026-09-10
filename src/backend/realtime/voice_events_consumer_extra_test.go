package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestVoiceEventBytesToFanout_DeclineMissedAndState(t *testing.T) {
	roomID := uuid.NewString()
	chatID := uuid.NewString()
	caller := uuid.NewString()
	callee := uuid.NewString()

	declined := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-decline",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallDeclined{
			CallDeclined: &eventsv1.CallDeclined{
				RoomId:              roomID,
				ChatId:              chatID,
				DeclinedByProfileId: callee,
				ProfileIds:          []string{caller, callee},
			},
		},
	}
	b, err := proto.Marshal(declined)
	if err != nil {
		t.Fatal(err)
	}
	profiles, fe, ok := voiceEventBytesToFanout(b)
	if !ok || fe.Op != "call_declined" || len(profiles) != 2 {
		t.Fatalf("declined ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}

	missed := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-missed",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallMissed{
			CallMissed: &eventsv1.CallMissed{
				RoomId:             roomID,
				ChatId:             chatID,
				InitiatorProfileId: caller,
				CalleeProfileId:    callee,
			},
		},
	}
	b, _ = proto.Marshal(missed)
	profiles, fe, ok = voiceEventBytesToFanout(b)
	if !ok || fe.Op != "call_missed" || len(profiles) != 2 {
		t.Fatalf("missed ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}

	muted := true
	state := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-state",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_VoiceStateChanged{
			VoiceStateChanged: &eventsv1.VoiceStateChanged{
				RoomId:     roomID,
				ProfileId:  caller,
				IsMuted:    &muted,
				ProfileIds: []string{caller, callee},
			},
		},
	}
	b, _ = proto.Marshal(state)
	profiles, fe, ok = voiceEventBytesToFanout(b)
	if !ok || fe.Op != "voice_state_update" || len(profiles) != 2 {
		t.Fatalf("state ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}
	var payload map[string]any
	if err := json.Unmarshal(fe.D, &payload); err != nil {
		t.Fatal(err)
	}
	if payload["is_muted"] != true {
		t.Fatalf("payload=%v", payload)
	}
}

func TestVoiceEventBytesToFanout_InvalidPayload(t *testing.T) {
	_, _, ok := voiceEventBytesToFanout([]byte("not-protobuf"))
	if ok {
		t.Fatal("expected invalid protobuf to be dropped")
	}
}

func TestVoiceEventBytesToFanout_CallStartedAndMemberJoined(t *testing.T) {
	roomID := uuid.NewString()
	chatID := uuid.NewString()
	owner := uuid.NewString()
	member := uuid.NewString()

	started := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-started",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallStarted{
			CallStarted: &eventsv1.CallStarted{
				RoomId:             roomID,
				ChatId:             chatID,
				InitiatorProfileId: owner,
				ProfileIds:         []string{owner},
				MediaKind:          "audio",
				LivekitRoomName:    "voice-group-" + roomID,
			},
		},
	}
	b, err := proto.Marshal(started)
	if err != nil {
		t.Fatal(err)
	}
	profiles, fe, ok := voiceEventBytesToFanout(b)
	if !ok || fe.Op != "call_started" || len(profiles) != 1 {
		t.Fatalf("started ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}

	joined := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-joined",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{
			VoiceMemberJoined: &eventsv1.VoiceMemberJoined{
				RoomId:           roomID,
				VoiceRoomId:      uuid.NewString(),
				SpaceId:          uuid.NewString(),
				JoinedProfileId:  member,
				NotifyProfileIds: []string{owner},
			},
		},
	}
	b, err = proto.Marshal(joined)
	if err != nil {
		t.Fatal(err)
	}
	profiles, fe, ok = voiceEventBytesToFanout(b)
	if !ok || fe.Op != "voice_member_joined" || len(profiles) != 1 || profiles[0] != owner {
		t.Fatalf("joined ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}
}

func TestVoiceEventBytesToFanout_CallStartedRoomBindingCompatibility(t *testing.T) {
	room, space, owner, member := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	for _, tc := range []struct {
		name, voiceRoom, space, kind string
		bound                        bool
	}{
		{"complete room", room, space, "voice_room", true},
		{"legacy event", "", "", "", false},
		{"missing space", room, "", "voice_room", false},
		{"missing room", "", space, "voice_room", false},
		{"malformed room", "bad", space, "voice_room", false},
		{"malformed space", room, "bad", "voice_room", false},
		{"direct incidental tuple", room, space, "call", false},
		{"group incidental tuple", room, space, "group_voice", false},
		{"tuple without kind", room, space, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := &eventsv1.CallStarted{RoomId: uuid.NewString(), ChatId: uuid.NewString(), InitiatorProfileId: owner, ProfileIds: []string{owner, member}, MediaKind: "audio", LivekitRoomName: "lk-binding"}
			if tc.voiceRoom != "" {
				started.VoiceRoomId = proto.String(tc.voiceRoom)
			}
			if tc.space != "" {
				started.SpaceId = proto.String(tc.space)
			}
			if tc.kind != "" {
				started.RoomType = proto.String(tc.kind)
			}
			wire, err := proto.Marshal(&eventsv1.VoiceStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.VoiceStreamEvent_CallStarted{CallStarted: started}})
			if err != nil {
				t.Fatal(err)
			}
			profiles, frame, ok := voiceEventBytesToFanout(wire)
			if !ok || frame.Op != "call_started" {
				t.Fatalf("ok=%v frame=%+v", ok, frame)
			}
			if !reflect.DeepEqual(profiles, []string{owner, member}) {
				t.Fatalf("audience changed: %v", profiles)
			}
			var payload map[string]any
			if err := json.Unmarshal(frame.D, &payload); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"room_id": started.RoomId, "chat_id": started.ChatId, "initiator_profile_id": owner, "callee_profile_id": "", "profile_ids": []any{owner, member}, "media_kind": "audio", "livekit_room_name": "lk-binding"}
			if tc.kind != "" {
				want["room_type"] = tc.kind
			}
			if tc.bound {
				want["voice_room_id"], want["space_id"] = room, space
			}
			if !reflect.DeepEqual(want, payload) {
				t.Fatalf("payload got=%v want=%v", payload, want)
			}
		})
	}
}

func TestVoiceEventBytesToFanout_MemberJoinedPreservesRoomBindingAndAudience(t *testing.T) {
	session, room, space, owner, joiner := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	wire, err := proto.Marshal(&eventsv1.VoiceStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.VoiceStreamEvent_VoiceMemberJoined{VoiceMemberJoined: &eventsv1.VoiceMemberJoined{RoomId: session, VoiceRoomId: room, SpaceId: space, JoinedProfileId: joiner, NotifyProfileIds: []string{owner}}}})
	if err != nil {
		t.Fatal(err)
	}
	profiles, frame, ok := voiceEventBytesToFanout(wire)
	if !ok || frame.Op != "voice_member_joined" || !reflect.DeepEqual(profiles, []string{owner}) {
		t.Fatalf("ok=%v op=%s audience=%v", ok, frame.Op, profiles)
	}
	var payload map[string]any
	if err := json.Unmarshal(frame.D, &payload); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"room_id": session, "voice_room_id": room, "space_id": space, "joined_profile_id": joiner, "profile_ids": []any{owner, joiner}}
	if !reflect.DeepEqual(want, payload) {
		t.Fatalf("payload got=%v want=%v", payload, want)
	}
}
