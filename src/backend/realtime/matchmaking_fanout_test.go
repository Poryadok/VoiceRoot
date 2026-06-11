package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func protoString(s string) *string { return &s }

func matchFoundPayload(t *testing.T, env fanoutEnvelope) map[string]string {
	t.Helper()
	if env.Op != "match_found" {
		t.Fatalf("op=%q want match_found", env.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(env.D, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestMatchmakingFanouts_MatchFoundEnvelope(t *testing.T) {
	matchID := uuid.NewString()
	gameID := uuid.NewString()
	profileA := uuid.NewString()
	profileB := uuid.NewString()
	sessionA := uuid.NewString()
	sessionB := uuid.NewString()

	ev := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchFound{
			MatchFound: &eventsv1.MatchFound{
				MatchId:    matchID,
				ProfileIds: []string{profileA, profileB},
				GameId:     gameID,
				Mode:       "Duo",
				Region:     "eu",
				SessionIds: []string{sessionA, sessionB},
				ChatId:      protoString("chat-1"),
				VoiceRoomId: protoString("voice-1"),
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := matchmakingFanouts(b)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 2 {
		t.Fatalf("fanouts=%d want 2", len(fanouts))
	}

	byProfile := map[string]map[string]string{}
	for _, f := range fanouts {
		byProfile[f.ProfileID] = matchFoundPayload(t, f.Envelope)
	}

	for _, profileID := range []string{profileA, profileB} {
		d, found := byProfile[profileID]
		if !found {
			t.Fatalf("missing profile %s", profileID)
		}
		if d["type"] != "match_found" {
			t.Fatalf("type=%q want match_found", d["type"])
		}
		if d["match_id"] != matchID || d["game_id"] != gameID || d["mode"] != "Duo" || d["region"] != "eu" {
			t.Fatalf("profile %s payload=%v", profileID, d)
		}
	}
}

func TestDispatchMatchmakingStreamEvent_MatchFoundFansOutPersonalOp(t *testing.T) {
	matchID := uuid.NewString()
	profileA := uuid.NewString()
	profileB := uuid.NewString()

	hub := newWSHub()
	regA := hub.attachConn("inst", "conn-a", profileA, 8)
	regB := hub.attachConn("inst", "conn-b", profileB, 8)

	ev := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchFound{
			MatchFound: &eventsv1.MatchFound{
				MatchId:    matchID,
				ProfileIds: []string{profileA, profileB},
				GameId:     uuid.NewString(),
				Mode:       "Duo",
				Region:     "eu",
			},
		},
	}
	payload, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	dispatchMatchmakingStreamEvent(hub, payload)

	opsA := drainFanoutOps(t, regA, 2*time.Second)
	opsB := drainFanoutOps(t, regB, 2*time.Second)
	if !containsOp(opsA, "match_found") || !containsOp(opsB, "match_found") {
		t.Fatalf("opsA=%v opsB=%v", opsA, opsB)
	}
}

func TestMatchmakingFanouts_InvalidProtobuf(t *testing.T) {
	fanouts, ok := matchmakingFanouts([]byte{0x01, 0x02})
	if ok || fanouts != nil {
		t.Fatalf("invalid protobuf: ok=%v fanouts=%v", ok, fanouts)
	}
}

func matchCompletedPayload(t *testing.T, env fanoutEnvelope) map[string]string {
	t.Helper()
	if env.Op != "match_completed" {
		t.Fatalf("op=%q want match_completed", env.Op)
	}
	var d map[string]string
	if err := json.Unmarshal(env.D, &d); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestMatchmakingFanouts_MatchCompletedEnvelope(t *testing.T) {
	matchID := uuid.NewString()
	profileA := uuid.NewString()
	profileB := uuid.NewString()

	ev := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchCompleted{
			MatchCompleted: &eventsv1.MatchCompleted{
				MatchId:         matchID,
				DurationSeconds: 1200,
				ProfileIds:      []string{profileA, profileB},
			},
		},
	}
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	fanouts, ok := matchmakingFanouts(b)
	if !ok {
		t.Fatal("expected ok")
	}
	if len(fanouts) != 2 {
		t.Fatalf("fanouts=%d want 2", len(fanouts))
	}

	byProfile := map[string]map[string]string{}
	for _, f := range fanouts {
		byProfile[f.ProfileID] = matchCompletedPayload(t, f.Envelope)
	}

	for _, profileID := range []string{profileA, profileB} {
		d, found := byProfile[profileID]
		if !found {
			t.Fatalf("missing profile %s", profileID)
		}
		if d["type"] != "match_completed" {
			t.Fatalf("type=%q want match_completed", d["type"])
		}
		if d["match_id"] != matchID {
			t.Fatalf("profile %s payload=%v", profileID, d)
		}
		if d["duration_seconds"] != "1200" {
			t.Fatalf("duration_seconds=%q want 1200", d["duration_seconds"])
		}
	}
}

func TestDispatchMatchmakingStreamEvent_MatchCompletedFansOutPersonalOp(t *testing.T) {
	matchID := uuid.NewString()
	profileA := uuid.NewString()
	profileB := uuid.NewString()

	hub := newWSHub()
	regA := hub.attachConn("inst", "conn-a", profileA, 8)
	regB := hub.attachConn("inst", "conn-b", profileB, 8)

	ev := &eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchCompleted{
			MatchCompleted: &eventsv1.MatchCompleted{
				MatchId:         matchID,
				DurationSeconds: 600,
				ProfileIds:      []string{profileA, profileB},
			},
		},
	}
	payload, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}

	dispatchMatchmakingStreamEvent(hub, payload)

	opsA := drainFanoutOps(t, regA, 2*time.Second)
	opsB := drainFanoutOps(t, regB, 2*time.Second)
	if !containsOp(opsA, "match_completed") || !containsOp(opsB, "match_completed") {
		t.Fatalf("opsA=%v opsB=%v", opsA, opsB)
	}
}
