package buffer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

// Flusher persists a batch of events.
type Flusher func(ctx context.Context, rows []store.EventRow) error

// Accumulator batches analytics events and flushes on size or interval.
type Accumulator struct {
	mu        sync.Mutex
	pending   []store.EventRow
	maxEvents int
	flushEvery time.Duration
	flusher   Flusher
	logger    *slog.Logger
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func New(maxEvents int, flushEvery time.Duration, flusher Flusher, logger *slog.Logger) *Accumulator {
	if maxEvents <= 0 {
		maxEvents = 1000
	}
	if flushEvery <= 0 {
		flushEvery = 5 * time.Second
	}
	return &Accumulator{
		maxEvents:  maxEvents,
		flushEvery: flushEvery,
		flusher:    flusher,
		logger:     logger,
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

func (a *Accumulator) Start(ctx context.Context) {
	go func() {
		defer close(a.doneCh)
		ticker := time.NewTicker(a.flushEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				_ = a.Flush(context.Background())
				return
			case <-a.stopCh:
				_ = a.Flush(context.Background())
				return
			case <-ticker.C:
				_ = a.Flush(ctx)
			}
		}
	}()
}

func (a *Accumulator) Stop() {
	close(a.stopCh)
	<-a.doneCh
}

func (a *Accumulator) AppendProto(ev *analyticsv1.AnalyticsEvent) {
	if a == nil || ev == nil {
		return
	}
	a.mu.Lock()
	a.pending = append(a.pending, store.RowFromProto(ev))
	shouldFlush := len(a.pending) >= a.maxEvents
	a.mu.Unlock()
	if shouldFlush {
		_ = a.Flush(context.Background())
	}
}

func (a *Accumulator) PendingCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.pending)
}

func (a *Accumulator) Flush(ctx context.Context) error {
	a.mu.Lock()
	if len(a.pending) == 0 {
		a.mu.Unlock()
		return nil
	}
	batch := a.pending
	a.pending = nil
	a.mu.Unlock()
	if a.flusher == nil {
		return nil
	}
	if err := a.flusher(ctx, batch); err != nil {
		if a.logger != nil {
			a.logger.Warn("analytics flush failed", slog.Any("error", err), slog.Int("batch_size", len(batch)))
		}
		a.mu.Lock()
		a.pending = append(batch, a.pending...)
		a.mu.Unlock()
		return err
	}
	return nil
}
