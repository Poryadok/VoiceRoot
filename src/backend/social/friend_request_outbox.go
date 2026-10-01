package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"voice/backend/social/internal/socialevents"
	"voice/backend/social/internal/store"
)

// runFriendRequestOutbox delivers committed invitation notifications independently
// of the client RPC and retries rows until JetStream acknowledges publication.
func runFriendRequestOutbox(ctx context.Context, friends *store.FriendshipStore, natsURL string, logger *slog.Logger) {
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
				logger.Warn("friend request outbox publisher unavailable", slog.Any("error", err))
			}
		}
		if publisher != nil {
			publisher.Logger = logger
			_, err := friends.DispatchFriendRequestOutbox(ctx, 25, func(ctx context.Context, requestID, eventID, requester, target uuid.UUID) error {
				return publisher.PublishFriendRequestWithEventID(ctx, eventID.String(), requestID.String(), requester.String(), target.String())
			})
			if err != nil && ctx.Err() == nil {
				logger.Warn("friend request outbox retrying", slog.Any("error", err))
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
