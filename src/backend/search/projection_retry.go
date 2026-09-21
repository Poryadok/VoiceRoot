package main

import (
	"context"
	"log/slog"
	"time"
)

// runProjectionWithRetry keeps required authority delivery alive through
// transient User/NATS startup failures without a hot retry loop.
func runProjectionWithRetry(ctx context.Context, logger *slog.Logger, name string, run func() error) {
	delay := time.Second
	for ctx.Err() == nil {
		err := run()
		if ctx.Err() != nil {
			return
		}
		logger.Warn(name+" failed; retrying", slog.Any("error", err), slog.Duration("delay", delay))
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		if delay < 30*time.Second {
			delay *= 2
		}
	}
}
