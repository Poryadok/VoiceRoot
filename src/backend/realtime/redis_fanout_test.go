package main

import (
	"context"
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisProfileRegistryRegisterUnregister(t *testing.T) {
	t.Parallel()
	s := miniredis.RunT(t)
	t.Cleanup(s.Close)

	rdb := redis.NewClient(&redis.Options{Addr: s.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	ctx := context.Background()
	inst := "instance-a"
	conn := "conn-1"
	prof := "profile-xyz"

	reg := newRedisProfileRegistry(rdb, "voice:rt:test:prof:")
	if err := reg.Register(ctx, prof, inst, conn); err != nil {
		t.Fatalf("Register: %v", err)
	}
	members, err := rdb.SMembers(ctx, "voice:rt:test:prof:"+prof).Result()
	if err != nil {
		t.Fatalf("SMembers: %v", err)
	}
	slices.Sort(members)
	want := []string{inst + ":" + conn}
	if len(members) != 1 || members[0] != want[0] {
		t.Fatalf("members = %v, want %v", members, want)
	}

	if err := reg.Unregister(ctx, prof, inst, conn); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	n, err := rdb.SCard(ctx, "voice:rt:test:prof:"+prof).Result()
	if err != nil {
		t.Fatalf("SCard: %v", err)
	}
	if n != 0 {
		t.Fatalf("SCard after unregister = %d, want 0", n)
	}
}

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

	regA := hub.attachConn(instA, connA, 8)
	regB := hub.attachConn(instB, connB, 8)
	hub.addChat(regA, chatID)
	hub.addChat(regB, chatID)

	rf := newRedisFanout(redisFanoutConfig{
		Client:        sub,
		Hub:           hub,
		InstanceID:    instB,
		KeyPrefix:     "voice:rt:test:prof:",
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
	reg := hub.attachConn(inst, connID, 8)
	hub.addChat(reg, "22222222-2222-2222-2222-222222222222")

	rf := newRedisFanout(redisFanoutConfig{
		Client:        rdb,
		Hub:           hub,
		InstanceID:    inst,
		KeyPrefix:     "voice:rt:test:prof:",
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
		KeyPrefix:     "voice:rt:test:prof:",
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
