package buffer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/store"
)

// Flusher persists a batch of events.
type Flusher func(ctx context.Context, rows []store.EventRow) error

// MsgAck supports deferred JetStream ack/nak after durable ClickHouse write.
type MsgAck interface {
	Ack(...nats.AckOpt) error
	Nak(...nats.AckOpt) error
}

type pendingEntry struct {
	row  store.EventRow
	acks []MsgAck
}

// Accumulator batches analytics events and flushes on size or interval.
type Accumulator struct {
	mu         sync.Mutex
	pending    []pendingEntry
	maxEvents  int
	flushEvery time.Duration
	flusher    Flusher
	logger     *slog.Logger
	stopCh     chan struct{}
	doneCh     chan struct{}
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
	a.AppendWithAck(ev, nil)
}

func (a *Accumulator) AppendWithAck(ev *analyticsv1.AnalyticsEvent, msg MsgAck) {
	if a == nil || ev == nil {
		if msg != nil {
			_ = msg.Ack()
		}
		return
	}
	entry := pendingEntry{row: store.RowFromProto(ev)}
	if msg != nil {
		entry.acks = []MsgAck{msg}
	}
	a.mu.Lock()
	a.pending = append(a.pending, entry)
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

	rows := make([]store.EventRow, len(batch))
	var acks []MsgAck
	for i, e := range batch {
		rows[i] = e.row
		acks = append(acks, e.acks...)
	}

	if a.flusher == nil {
		for _, ack := range acks {
			_ = ack.Ack()
		}
		return nil
	}
	if err := a.flusher(ctx, rows); err != nil {
		if a.logger != nil {
			a.logger.Warn("analytics flush failed", slog.Any("error", err), slog.Int("batch_size", len(batch)))
		}
		for _, ack := range acks {
			_ = ack.Nak()
		}
		if len(acks) == 0 {
			a.mu.Lock()
			a.pending = append(batch, a.pending...)
			a.mu.Unlock()
		}
		return err
	}
	for _, ack := range acks {
		_ = ack.Ack()
	}
	return nil
}
