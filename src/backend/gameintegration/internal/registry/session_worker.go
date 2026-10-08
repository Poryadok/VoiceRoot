package registry

import (
	"context"
	"time"
)

// RunSessionWorker polls durable pending operations. The database lease is the
// cross-process single-worker guard; cancellation never erases a claimed stage.
func RunSessionWorker(ctx context.Context, orchestrator *SessionOrchestrator, interval time.Duration) {
	if interval <= 0 {
		interval = 250 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		keys, err := orchestrator.Store.due(ctx)
		if err == nil {
			for _, key := range keys {
				if ctx.Err() != nil {
					return
				}
				_, _ = orchestrator.AdvanceScoped(ctx, key)
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
