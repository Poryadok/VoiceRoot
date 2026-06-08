package main

import (
	"encoding/json"
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
