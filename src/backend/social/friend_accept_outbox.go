package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"voice/backend/social/internal/socialevents"
	"voice/backend/social/internal/store"
)

// runFriendAcceptanceOutbox keeps the friendship-to-DM transition deliverable
// when NATS is unavailable at acceptance time. The store commits the outbox
// row with the friendship and retries publishing until delivery is recorded.
func runFriendAcceptanceOutbox(ctx context.Context, friends *store.FriendshipStore, natsURL string, logger *slog.Logger) {
	var publisher *socialevents.JetStreamPublisher
	defer func() {
		if publisher != nil {
			_ = publisher.Close()
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		if publisher == nil {
			var err error
			publisher, err = socialevents.NewJetStreamPublisher(natsURL)
			if err != nil {
				logger.Warn("friend acceptance outbox publisher unavailable", slog.Any("error", err))
			}
		}
		if publisher != nil {
			publisher.Logger = logger
			_, err := friends.DispatchAcceptedFriendOutbox(ctx, 25, func(ctx context.Context, requester, target uuid.UUID) error {
				return publisher.PublishFriendAccepted(ctx, requester.String(), target.String())
			})
			if err != nil && ctx.Err() == nil {
				logger.Warn("friend acceptance outbox retrying", slog.Any("error", err))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
