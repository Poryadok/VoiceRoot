package jobs

import (
	"context"
	"log/slog"
	"time"

	"voice/backend/story/internal/store"
)

// StartExpiryWorker marks stories expired past TTL every minute.
func StartExpiryWorker(ctx context.Context, st *store.StoryStore, logger *slog.Logger) {
	if st == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := st.MarkExpiredStories(context.Background(), time.Now().UTC())
				if err != nil && logger != nil {
					logger.Error("story expiry worker", slog.String("error", err.Error()))
				} else if n > 0 && logger != nil {
					logger.Info("story expiry worker", slog.Int64("expired", n))
				}
			}
		}
	}()
}

// StartArchivePurgeWorker deletes archived stories past retention daily.
func StartArchivePurgeWorker(ctx context.Context, st *store.StoryStore, logger *slog.Logger) {
	if st == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				n, err := st.PurgeArchivedStories(context.Background(), time.Now().UTC())
				if err != nil && logger != nil {
					logger.Error("story archive purge", slog.String("error", err.Error()))
				} else if n > 0 && logger != nil {
					logger.Info("story archive purge", slog.Int64("purged", n))
				}
			}
		}
	}()
}
