package main

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

const maxJetStreamSubscribeRetryDelay = 5 * time.Second

func isJetStreamNotFound(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, nats.ErrStreamNotFound) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "stream not found")
}

func nextJetStreamSubscribeRetryDelay(delay time.Duration) time.Duration {
	if delay >= maxJetStreamSubscribeRetryDelay {
		return maxJetStreamSubscribeRetryDelay
	}
	delay *= 2
	if delay > maxJetStreamSubscribeRetryDelay {
		return maxJetStreamSubscribeRetryDelay
	}
	return delay
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
		svcLogger.Info("jetstream stream not ready, retrying",
			slog.String("component", logPrefix),
			slog.Duration("retry_in", delay),
			slog.String("error", err.Error()),
		)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
		delay = nextJetStreamSubscribeRetryDelay(delay)
	}
}
