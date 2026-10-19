package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisTypingFanoutCrossInstanceViaPubSub(t *testing.T) {
	t.Parallel()
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)

	addr := s.Addr()
	pub := redis.NewClient(&redis.Options{Addr: addr})
	sub := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = pub.Close(); _ = sub.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub := newWSHub()
	instA := "inst-a"
	instB := "inst-b"
	connA := "c-a"
	connB := "c-b"
	chatID := "11111111-1111-1111-1111-111111111111"

	regA := hub.attachConn(instA, connA, "", 8)
	regB := hub.attachConn(instB, connB, "", 8)
	hub.addChat(regA, chatID)
	hub.addChat(regB, chatID)

	rf := newRedisFanout(redisFanoutConfig{
		Client:        sub,
		Hub:           hub,
		InstanceID:    instB,
		FanoutChannel: "voice:rt:test:fanout",
	})
	go func() { _ = rf.runSubscriber(ctx) }()

	time.Sleep(50 * time.Millisecond)

	msg := redisFanoutPayload{
		ChatID:      chatID,
		ProfileID:   "profile-p",
		Kind:        "start",
		SrcInstance: instA,
		SrcConn:     connA,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := pub.Publish(ctx, "voice:rt:test:fanout", string(b)).Err(); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case env := <-regB.fanout:
		if env.Op != "typing" {
			t.Fatalf("op = %q, want typing", env.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for fanout delivery to conn B")
	}
}

func TestRedisFanoutPublishRoundTrip(t *testing.T) {
	t.Parallel()
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub := newWSHub()
	inst := "inst-1"
	connID := uuid.NewString()
	reg := hub.attachConn(inst, connID, "", 8)
	hub.addChat(reg, "22222222-2222-2222-2222-222222222222")

	rf := newRedisFanout(redisFanoutConfig{
		Client:        rdb,
		Hub:           hub,
		InstanceID:    inst,
		FanoutChannel: "voice:rt:test:fanout2",
	})
	go func() { _ = rf.runSubscriber(ctx) }()
	time.Sleep(50 * time.Millisecond)

	publisher := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = publisher.Close() })
	pubRF := newRedisFanout(redisFanoutConfig{
		Client:        publisher,
		Hub:           hub,
		InstanceID:    inst,
		FanoutChannel: "voice:rt:test:fanout2",
	})

	if err := pubRF.PublishTyping(context.Background(), "22222222-2222-2222-2222-222222222222", "p1", "start", connID); err != nil {
		t.Fatalf("PublishTyping: %v", err)
	}

	// Same connection is excluded from local delivery; should not receive own typing.
	select {
	case <-reg.fanout:
		t.Fatal("publisher should not receive own typing on same conn")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestRedisDeliveryAckFanoutCrossInstance(t *testing.T) {
	t.Parallel()
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)

	addr := s.Addr()
	pub := redis.NewClient(&redis.Options{Addr: addr})
	sub := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = pub.Close(); _ = sub.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	hub := newWSHub()
	senderConn := hub.attachConn("inst-b", "sender-conn", "sender-profile", 8)

	rf := newRedisFanout(redisFanoutConfig{
		Client:        sub,
		Hub:           hub,
		InstanceID:    "inst-b",
		FanoutChannel: "voice:rt:test:fanout:delivery",
	})
	go func() { _ = rf.runSubscriber(ctx) }()
	time.Sleep(50 * time.Millisecond)

	pubRF := newRedisFanout(redisFanoutConfig{
		Client:        pub,
		Hub:           hub,
		InstanceID:    "inst-a",
		FanoutChannel: "voice:rt:test:fanout:delivery",
	})
	chatID := "33333333-3333-3333-3333-333333333333"
	msgID := "44444444-4444-4444-4444-444444444444"
	if err := pubRF.PublishDeliveryAck(context.Background(), "sender-profile", chatID, msgID, "recipient-profile", "conn-a"); err != nil {
		t.Fatalf("PublishDeliveryAck: %v", err)
	}

	select {
	case env := <-senderConn.fanout:
		if env.Op != "message_delivered" {
			t.Fatalf("op = %q, want message_delivered", env.Op)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for delivery ack fanout")
	}
}
