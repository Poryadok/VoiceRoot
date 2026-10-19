package jobs

import (
	"context"
	"log/slog"
	"time"
)

type previewOutbox interface {
	DispatchPreviewEvent(context.Context, func(context.Context, []byte) error) (bool, error)
}
type previewPublisher interface {
	PublishPreviewEvent(context.Context, []byte) error
}

// Activation is explicit: all Messaging replicas must understand preview first.
func StartPreviewEventWorker(ctx context.Context, store previewOutbox, publisher previewPublisher, logger *slog.Logger) {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			for i := 0; i < 16; i++ {
				published, err := store.DispatchPreviewEvent(ctx, publisher.PublishPreviewEvent)
				if err != nil {
					logger.WarnContext(ctx, "file preview publication pending")
					break
				}
				if !published {
					break
				}
			}
		}
	}()
}
