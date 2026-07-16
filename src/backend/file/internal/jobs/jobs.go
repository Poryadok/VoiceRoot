package jobs

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"voice/backend/file/internal/fileevents"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

const expiryBatchSize = 100

// RunExpiryPurgeOnce deletes R2 objects for expired ready files, marks them expired, and publishes events.
func RunExpiryPurgeOnce(
	ctx context.Context,
	files *store.FilesStore,
	deleter r2file.ObjectDeleter,
	pub fileevents.Publisher,
	now time.Time,
) (int64, error) {
	if files == nil || files.Pool == nil {
		return 0, nil
	}
	rows, err := files.ListExpiredReadyFiles(ctx, now.UTC(), expiryBatchSize)
	if err != nil {
		return 0, err
	}
	var processed int64
	for _, row := range rows {
		if deleter != nil {
			if err := r2file.DeleteKeys(ctx, deleter, FileStorageKeys(row)...); err != nil {
				return processed, err
			}
		}
		if err := files.MarkExpired(ctx, row.ID); err != nil {
			return processed, err
		}
		if pub != nil {
			var chatID *string
			if row.ChatID != nil {
				s := row.ChatID.String()
				chatID = &s
			}
			_ = pub.PublishFileExpired(ctx, row.ID.String(), chatID)
		}
		processed++
	}
	return processed, nil
}

// StartExpiryWorker purges expired files every minute.
func StartExpiryWorker(
	ctx context.Context,
	files *store.FilesStore,
	deleter r2file.ObjectDeleter,
	pub fileevents.Publisher,
	logger *slog.Logger,
) {
	if files == nil {
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
				n, err := RunExpiryPurgeOnce(context.Background(), files, deleter, pub, time.Now().UTC())
				if err != nil && logger != nil {
					logger.Error("file expiry worker", slog.String("error", err.Error()))
					continue
				}
				if n > 0 && logger != nil {
					logger.Info("file expiry worker", slog.Int64("expired", n))
				}
			}
		}
	}()
}

// FileStorageKeys returns unique R2 object keys associated with a file row.
func FileStorageKeys(row store.FileRow) []string {
	keys := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(raw string) {
		key := strings.TrimSpace(raw)
		if key == "" {
			return
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}
	add(row.R2Key)
	if row.ConvertedR2Key != nil {
		add(*row.ConvertedR2Key)
	}
	if row.ThumbnailR2Key != nil {
		add(*row.ThumbnailR2Key)
	}
	return keys
}
