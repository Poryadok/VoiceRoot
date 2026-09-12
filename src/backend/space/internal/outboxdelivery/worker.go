package outboxdelivery

import (
	"context"
	"errors"
	"time"
)

type dispatcher interface {
	DispatchOnce(context.Context) error
}

// Worker serializes bounded dispatch passes. Cancellation is passed to the
// current pass, and Run waits for that pass to return before it exits.
type Worker struct {
	dispatcher dispatcher
	interval   time.Duration
	onError    func(error)
}

func NewWorker(dispatcher dispatcher, interval time.Duration, onError ...func(error)) *Worker {
	worker := &Worker{dispatcher: dispatcher, interval: interval}
	if len(onError) != 0 {
		worker.onError = onError[0]
	}
	return worker
}

func (w *Worker) Run(ctx context.Context) error {
	if w == nil || w.dispatcher == nil || w.interval <= 0 {
		return errors.New("ownership outbox worker not configured")
	}
	if ctx == nil {
		return errors.New("ownership outbox worker context is nil")
	}
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		if err := ctx.Err(); err != nil {
			return nil
		}
		if err := w.dispatcher.DispatchOnce(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if w.onError != nil {
				w.onError(err)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
