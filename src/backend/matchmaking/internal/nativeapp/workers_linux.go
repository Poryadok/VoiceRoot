//go:build linux

package nativeapp

import (
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"log/slog"
	"sync"
	"time"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/squad"
	"voice/backend/matchmaking/internal/timeout"
)

type nativeWorkers struct {
	cancel context.CancelFunc
	done   sync.WaitGroup
	once   sync.Once
}

func (w *nativeWorkers) Close() {
	if w == nil {
		return
	}
	w.once.Do(func() { w.cancel(); w.done.Wait() })
}

func startNativeWorkers(parent context.Context, runtime *preparedRuntime, logger *slog.Logger) *nativeWorkers {
	ctx, cancel := context.WithCancel(parent)
	workers := &nativeWorkers{cancel: cancel}
	start := func(name string, interval, budget time.Duration, forward bool, run func(context.Context) error) {
		workers.done.Add(1)
		go func() {
			defer workers.done.Done()
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
				attempt, stop := context.WithTimeout(ctx, budget)
				var release func()
				var err error
				if forward {
					release, err = runtime.gate.EnterForward(attempt, uuid.New(), sha256.Sum256([]byte("voice.mm.native.worker.v1\x00"+name)))
				}
				if err == nil {
					err = run(attempt)
				}
				// Registration covers the complete attempt, including durable
				// failure classification; context cancellation never releases it.
				if release != nil {
					release()
				}
				stop()
				if err != nil && ctx.Err() == nil {
					logger.Warn("native worker attempt unavailable", slog.String("worker", name))
				}
			}
		}()
	}
	start("matcher", 2*time.Second, 30*time.Second, true, runtime.matcher.RunOnce)
	start("projector", 250*time.Millisecond, 12*time.Second, false, func(ctx context.Context) error { _, err := runtime.projector.RunOnce(ctx); return err })
	start("provision", 2*time.Second, 60*time.Second, false, func(ctx context.Context) error {
		_, err := runtime.provider.Worker.RunProvisioningOnce(ctx, 100)
		if errors.Is(err, squad.ErrProvisionOperatorAttention) {
			logger.Error("native provisioning requires operator attention")
		}
		return err
	})
	// Issued obligations retain their original protected request and can settle
	// while admission is closed. Never wrap these workers in a fresh roster gate.
	start("teardown", 2*time.Second, 60*time.Second, false, func(ctx context.Context) error {
		_, err := runtime.provider.Worker.RunTeardownOnce(ctx, 100)
		return err
	})
	start("compaction", 2*time.Second, 30*time.Second, false, func(ctx context.Context) error {
		_, err := runtime.provider.Worker.RunCompactionOnce(ctx, 100)
		return err
	})
	start("timeouts", 30*time.Second, 20*time.Second, false, runtime.sweeper.RunNativeOnce)
	start("timeout-events", time.Second, 10*time.Second, false, func(ctx context.Context) error {
		return timeout.RelayNativeTimeouts(ctx, runtime.service.Matches, runtime.events)
	})
	start("completion-events", 2*time.Second, 10*time.Second, false, func(ctx context.Context) error {
		pending, err := runtime.service.Matches.ListPendingMatchSquadCompletionEvents(ctx, 100)
		if err != nil {
			return err
		}
		for _, event := range pending {
			if err := runtime.events.PublishMatchCompleted(ctx, mmevents.MatchCompletedEvent{EventID: event.EventID.String(), OccurredAt: event.OccurredAt, MatchID: event.MatchID.String(), DurationSeconds: event.DurationSeconds, ProfileIDs: event.ProfileIDs}); err != nil {
				return err
			}
			if err := runtime.service.Matches.MarkMatchSquadCompletionEventPublished(ctx, event.AggregateID, event.EventID); err != nil {
				return err
			}
		}
		return nil
	})
	return workers
}
