package main

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

func isJetStreamNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, nats.ErrStreamNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "stream not found")
}

func subscribeJetStreamWithRetry(
	ctx context.Context,
	logPrefix string,
	subscribe func() (*nats.Subscription, error),
) (*nats.Subscription, error) {
	delay := time.Second
	for {
		sub, err := subscribe()
		if err == nil {
			return sub, nil
		}
		if !isJetStreamNotFound(err) {
			return nil, err
		}
		log.Printf("%s: jetstream stream not ready, retry in %s: %v", logPrefix, delay, err)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}
