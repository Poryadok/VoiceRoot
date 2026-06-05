package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
)

func TestIsJetStreamNotFound(t *testing.T) {
	if !isJetStreamNotFound(nats.ErrStreamNotFound) {
		t.Fatal("expected ErrStreamNotFound")
	}
	if !isJetStreamNotFound(errors.New("jetstream subscribe message.events: nats: stream not found")) {
		t.Fatal("expected wrapped stream not found")
	}
	if isJetStreamNotFound(errors.New("connection refused")) {
		t.Fatal("unexpected match")
	}
}

func TestSubscribeJetStreamWithRetry_WaitsForStream(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	attempts := 0
	sub, err := subscribeJetStreamWithRetry(ctx, "test", func() (*nats.Subscription, error) {
		attempts++
		if attempts < 3 {
			return nil, nats.ErrStreamNotFound
		}
		return &nats.Subscription{}, nil
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription")
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
}
