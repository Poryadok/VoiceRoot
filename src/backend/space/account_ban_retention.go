package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

// The store repeats the catalog, lifecycle, execution and strict completed30day
// predicates under its Space fence. Scheduler hints never authorize deletion.
type accountBanRetentionStore interface {
	ListExpiredAccountBanIntentSpaces(context.Context, int) ([]uuid.UUID, error)
	CleanupExpiredAccountBanIntents(context.Context, uuid.UUID, int) (int64, error)
}

type accountBanRetentionRuntime struct {
	cancel context.CancelFunc
	done   <-chan struct{}
}

func startAccountBanRetentionRuntime(parent context.Context, st accountBanRetentionStore, logger *slog.Logger) *accountBanRetentionRuntime {
	if st == nil {
		return nil
	}
	ticker := time.NewTicker(time.Minute)
	return startAccountBanRetentionWithTicks(parent, st, logger, ticker.C, ticker.Stop)
}

func startAccountBanRetentionWithTicks(parent context.Context, st accountBanRetentionStore, logger *slog.Logger, ticks <-chan time.Time, stopTicks func()) *accountBanRetentionRuntime {
	ctx, cancel := context.WithCancel(parent)
	done := make(chan struct{})
	runtime := &accountBanRetentionRuntime{cancel: cancel, done: done}
	go func() {
		defer close(done)
		defer stopTicks()
		for {
			if ctx.Err() != nil {
				return
			}
			runAccountBanRetentionCycle(ctx, st, logger)
			// A tick observed while a cycle was busy cannot queue another cycle.
			for draining := true; draining; {
				select {
				case _, ok := <-ticks:
					if !ok {
						return
					}
				default:
					draining = false
				}
			}
			select {
			case <-ctx.Done():
				return
			case _, ok := <-ticks:
				if !ok {
					return
				}
			}
		}
	}()
	return runtime
}

func (r *accountBanRetentionRuntime) Stop() {
	if r == nil {
		return
	}
	r.cancel()
	<-r.done
}

func runAccountBanRetentionCycle(parent context.Context, st accountBanRetentionStore, logger *slog.Logger) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	failed := false
	spaces, err := st.ListExpiredAccountBanIntentSpaces(ctx, 16)
	if err != nil {
		failed = true
	} else {
		if len(spaces) > 16 {
			spaces = spaces[:16]
		}
		for _, space := range spaces {
			if ctx.Err() != nil {
				break
			}
			if _, err = st.CleanupExpiredAccountBanIntents(ctx, space, 256); err != nil {
				failed = true
			}
		}
	}
	if parent.Err() != nil {
		return
	} // Shutdown cancellation is silent.
	if failed || ctx.Err() != nil {
		failure := "database"
		if ctx.Err() == context.DeadlineExceeded {
			failure = "deadline"
		}
		logger.Warn("Space account Ban outcome retention cycle failed", slog.String("failure", failure))
	}
}
