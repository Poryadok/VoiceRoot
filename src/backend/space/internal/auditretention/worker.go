// Package auditretention runs bounded cleanup passes for delivered Space audit
// effects whose database timestamps are outside the 365-day retention window.
package auditretention

import (
	"context"
	"errors"
	"time"
)

// Cleaner is implemented by store.SpaceStore. The store owns the database-time
// cutoff and never deletes an undelivered outbox effect.
type Cleaner interface {
	CleanupExpiredAuditLog(context.Context, int) (int64, error)
}

// Worker performs bounded cleanup batches. Runtime wiring may call Run or
// schedule RunOnce with the service's existing job coordinator.
type Worker struct {
	Cleaner   Cleaner
	BatchSize int
	Interval  time.Duration
}

// RunOnce drains full batches and stops after the first partial batch.
func (w Worker) RunOnce(ctx context.Context) (int64, error) {
	if w.Cleaner == nil || w.BatchSize < 1 {
		return 0, errors.New("invalid audit retention worker")
	}
	var total int64
	for {
		deleted, err := w.Cleaner.CleanupExpiredAuditLog(ctx, w.BatchSize)
		if err != nil {
			return total, err
		}
		total += deleted
		if deleted < int64(w.BatchSize) {
			return total, nil
		}
	}
}

// Run performs one immediate cleanup and then repeats until cancellation.
func (w Worker) Run(ctx context.Context) error {
	if w.Interval <= 0 {
		return errors.New("invalid audit retention interval")
	}
	if _, err := w.RunOnce(ctx); err != nil {
		return err
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-ticker.C:
			if _, err := w.RunOnce(ctx); err != nil {
				return err
			}
		}
	}
}
