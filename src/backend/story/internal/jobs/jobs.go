package jobs

import (
	"context"
	"log/slog"
	"time"

	"voice/backend/story/internal/store"
	"voice/backend/story/internal/storyevents"
)

// FileDeleter removes story media from object storage (File Service).
type FileDeleter interface {
	DeleteFile(ctx context.Context, fileID string) error
}

// RunArchivePurgeOnce commits logical deletion before any external File call, then
// dispatches durable media operations. File deletion is at-least-once by file id.
func RunArchivePurgeOnce(ctx context.Context, st *store.StoryStore, deleter FileDeleter, now time.Time) (int64, error) {
	if st == nil || st.Pool == nil {
		return 0, nil
	}
	batch, err := st.StageArchivePurgeBatch(ctx, 100)
	if err != nil {
		return 0, err
	}
	if deleter == nil {
		return batch.Stories, nil
	}
	ops, err := st.ClaimMediaDeletion(ctx, 100, time.Minute)
	if err != nil {
		return 0, err
	}
	for _, op := range ops {
		if err := deleter.DeleteFile(ctx, op.MediaFileID.String()); err != nil {
			_, _ = st.FailMediaDeletion(ctx, op.OperationID, op.LeaseToken, err)
			continue
		}
		_, err := st.CompleteMediaDeletion(ctx, op.OperationID, op.LeaseToken)
		if err != nil {
			return 0, err
		}
	}
	return batch.Stories, nil
}

// StartExpiryWorker marks stories expired past TTL every minute and publishes story.expired.
func StartExpiryWorker(ctx context.Context, st *store.StoryStore, pub storyevents.Publisher, logger *slog.Logger) {
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
				ids, n, err := st.MarkExpiredStoriesReturning(context.Background(), time.Now().UTC())
				if err != nil && logger != nil {
					logger.Error("story expiry worker", slog.String("error", err.Error()))
					continue
				}
				if pub != nil {
					for _, id := range ids {
						_ = pub.PublishStoryExpired(context.Background(), id.String())
					}
				}
				if n > 0 && logger != nil {
					logger.Info("story expiry worker", slog.Int64("expired", n))
				}
			}
		}
	}()
}

// StartArchivePurgeWorker deletes archived stories past retention daily.
func StartArchivePurgeWorker(ctx context.Context, st *store.StoryStore, deleter FileDeleter, logger *slog.Logger) {
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
				n, err := RunArchivePurgeOnce(context.Background(), st, deleter, time.Now().UTC())
				if err != nil && logger != nil {
					logger.Error("story archive purge", slog.String("error", err.Error()))
				} else if n > 0 && logger != nil {
					logger.Info("story archive purge", slog.Int64("purged", n))
				}
			}
		}
	}()
}
