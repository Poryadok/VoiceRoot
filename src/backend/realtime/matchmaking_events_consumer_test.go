package main

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
)

func TestRunMatchmakingEventsConsumer_JetStreamToProfile(t *testing.T) {
	s := startRealtimeJSTestServer(t)
	natsURL := s.ClientURL()

	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = nc.Drain() }()
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.AddStream(&nats.StreamConfig{
		Name:      jsStreamMatchmakingEvents,
		Subjects:  []string{"mm.>"},
		Retention: nats.LimitsPolicy,
	}); err != nil {
		t.Fatalf("add matchmaking event stream: %v", err)
	}

	const instanceID = "matchmaking-consumer-test"
	preprovisionRealtimeConsumer(t, js, jsStreamMatchmakingEvents, matchmakingConsumerDurableName(instanceID), "mm.>")

	hub := newWSHub()
	recipientProfileID := uuid.NewString()
	nonRecipientProfileID := uuid.NewString()
	recipient := hub.attachConn("inst", "matchmaking-recipient", recipientProfileID, 8)
	nonRecipient := hub.attachConn("inst", "matchmaking-non-recipient", nonRecipientProfileID, 8)

	sub, err := subscribeMatchmakingEvents(js, hub, instanceID, nil)
	if err != nil {
		t.Fatalf("subscribe matchmaking events: %v", err)
	}
	defer func() { _ = sub.Unsubscribe() }()

	matchID := uuid.NewString()
	eventBytes, err := proto.Marshal(&eventsv1.MatchmakingStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.MatchmakingStreamEvent_MatchFound{
			MatchFound: &eventsv1.MatchFound{
				MatchId:    matchID,
				ProfileIds: []string{recipientProfileID},
				GameId:     uuid.NewString(),
				Mode:       "Duo",
				Region:     "eu",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal match-found event: %v", err)
	}
	pubAck, err := js.Publish("mm.match_found", eventBytes)
	if err != nil {
		t.Fatalf("publish match-found event: %v", err)
	}

	select {
	case env := <-recipient.fanout:
		if env.Op != "match_found" {
			t.Fatalf("recipient op=%q want match_found", env.Op)
		}
		var payload map[string]string
		if err := json.Unmarshal(env.D, &payload); err != nil {
			t.Fatalf("decode recipient payload: %v", err)
		}
		if payload["type"] != "match_found" || payload["match_id"] != matchID {
			t.Fatalf("recipient payload=%v, want match_found %s", payload, matchID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for recipient match_found fan-out")
	}

	// subscribeMatchmakingEvents uses the default async auto-ack behavior. The nats.go wrapper
	// sends the ACK only after the callback returns, so the consumer ack floor is a completion
	// barrier for dispatchMatchmakingStreamEvent, not just evidence that delivery began.
	ackFloorTicker := time.NewTicker(10 * time.Millisecond)
	defer ackFloorTicker.Stop()
	ackFloorTimeout := time.NewTimer(5 * time.Second)
	defer ackFloorTimeout.Stop()
	for {
		info, err := js.ConsumerInfo(jsStreamMatchmakingEvents, matchmakingConsumerDurableName(instanceID))
		if err != nil {
			t.Fatalf("read matchmaking consumer ack floor: %v", err)
		}
		if info.AckFloor.Stream >= pubAck.Sequence {
			break
		}
		select {
		case <-ackFloorTicker.C:
		case <-ackFloorTimeout.C:
			t.Fatalf("timed out waiting for matchmaking callback ACK: ack floor=%d published sequence=%d", info.AckFloor.Stream, pubAck.Sequence)
		}
	}

	select {
	case env := <-nonRecipient.fanout:
		t.Fatalf("non-recipient received matchmaking event: %+v", env)
	default:
	}

}
