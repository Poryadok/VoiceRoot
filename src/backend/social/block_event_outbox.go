package main

import (
	"context"
	"log/slog"
	"time"

	"voice/backend/social/internal/socialevents"
	"voice/backend/social/internal/store"
)

// This worker starts even when the RPC publisher cannot connect at startup.
func runBlockEventOutbox(ctx context.Context, blocks *store.BlockStore, natsURL string, logger *slog.Logger) {
	var publisher *socialevents.JetStreamPublisher
	defer func() {
		if publisher != nil {
			_ = publisher.Close()
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if publisher == nil {
			var err error
			publisher, err = socialevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				logger.Warn("block outbox publisher unavailable", slog.Any("error", err))
			}
		}
		if publisher != nil {
			_, err := blocks.DispatchBlockEventOutbox(ctx, 25, publisher.PublishBlockOutboxEvent)
			if err != nil && ctx.Err() == nil {
				logger.Warn("block outbox retrying", slog.Any("error", err))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
