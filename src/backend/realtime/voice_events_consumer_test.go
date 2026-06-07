package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestVoiceEventBytesToFanout_IncomingAcceptedEnded(t *testing.T) {
	roomID := uuid.NewString()
	chatID := uuid.NewString()
	caller := uuid.NewString()
	callee := uuid.NewString()

	incoming := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-1",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallIncoming{
			CallIncoming: &eventsv1.CallIncoming{
				RoomId:             roomID,
				ChatId:             chatID,
				InitiatorProfileId: caller,
				CalleeProfileId:    callee,
				MediaKind:          "video",
				LivekitRoomName:    "voice-dm-" + roomID,
				ExpiresAt:          timestamppb.New(time.Unix(1700000030, 0)),
			},
		},
	}
	b, err := proto.Marshal(incoming)
	if err != nil {
		t.Fatal(err)
	}
	profiles, fe, ok := voiceEventBytesToFanout(b)
	if !ok || fe.Op != "call_incoming" {
		t.Fatalf("incoming ok=%v op=%q", ok, fe.Op)
	}
	if len(profiles) != 1 || profiles[0] != callee {
		t.Fatalf("incoming profiles=%v", profiles)
	}
	var d map[string]any
	if err := json.Unmarshal(fe.D, &d); err != nil {
		t.Fatal(err)
	}
	if d["room_id"] != roomID || d["chat_id"] != chatID || d["media_kind"] != "video" {
		t.Fatalf("incoming payload=%v", d)
	}

	accepted := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-2",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallAccepted{
			CallAccepted: &eventsv1.CallAccepted{
				RoomId:              roomID,
				ChatId:              chatID,
				AcceptedByProfileId: callee,
				ProfileIds:          []string{caller, callee},
				MediaKind:           "video",
				LivekitRoomName:     "voice-dm-" + roomID,
			},
		},
	}
	b, _ = proto.Marshal(accepted)
	profiles, fe, ok = voiceEventBytesToFanout(b)
	if !ok || fe.Op != "call_accepted" || len(profiles) != 2 {
		t.Fatalf("accepted ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}

	ended := &eventsv1.VoiceStreamEvent{
		EventId:    "voice-3",
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallEnded{
			CallEnded: &eventsv1.CallEnded{
				RoomId:           roomID,
				DurationSeconds:  12,
				ProfileIds:       []string{caller, callee},
				Reason:           "hangup",
				EndedByProfileId: caller,
			},
		},
	}
	b, _ = proto.Marshal(ended)
	profiles, fe, ok = voiceEventBytesToFanout(b)
	if !ok || fe.Op != "call_ended" || len(profiles) != 2 {
		t.Fatalf("ended ok=%v op=%q profiles=%v", ok, fe.Op, profiles)
	}
}

func TestRunVoiceEventsConsumer_JetStreamToProfileHub(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatalf("jetstream: %v", err)
	}
	_, err = js.AddStream(&nats.StreamConfig{
		Name:      jsStreamVoiceEvents,
		Subjects:  []string{"voice.>"},
		Retention: nats.LimitsPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   nats.FileStorage,
	})
	if err != nil {
		t.Fatalf("add stream: %v", err)
	}

	callee := "22222222-2222-2222-2222-222222222222"
	hub := newWSHub()
	reg := hub.attachConn("test-inst", "conn-1", callee, 8)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- runVoiceEventsConsumer(ctx, hub, natsURL, "voice-consumer-inst", nil) }()
	time.Sleep(200 * time.Millisecond)

	env := &eventsv1.VoiceStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.VoiceStreamEvent_CallIncoming{
			CallIncoming: &eventsv1.CallIncoming{
				RoomId:             "room-1",
				ChatId:             "chat-1",
				InitiatorProfileId: "profile-1",
				CalleeProfileId:    callee,
				MediaKind:          "audio",
			},
		},
	}
	payload, err := proto.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.Publish("voice.call_incoming", payload); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case fe := <-reg.fanout:
		if fe.Op != "call_incoming" {
			t.Fatalf("op=%q", fe.Op)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("timeout waiting for call fan-out")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil && err != context.Canceled {
			t.Fatalf("consumer err=%v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("consumer did not stop")
	}
}
