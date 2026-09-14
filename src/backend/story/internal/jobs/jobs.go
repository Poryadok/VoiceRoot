package jobs

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"sync"
	"time"

	"voice/backend/story/internal/store"
	"voice/backend/story/internal/storyevents"
)

// FileDeleter removes story media from object storage (File Service).
type FileDeleter interface {
	DeleteFile(ctx context.Context, fileID string) error
}

const defaultArchivePurgeFileTimeout = 15 * time.Second

var (
	archivePurgeFileCallGates sync.Map
	errArchivePurgeFileBusy   = errors.New("archive purge File deletion already in flight")
)

// RunArchivePurgeOnce commits logical deletion before any external File call, then
// dispatches durable media operations. File deletion is at-least-once by file id.
func RunArchivePurgeOnce(ctx context.Context, st *store.StoryStore, deleter FileDeleter, now time.Time) (int64, error) {
	return RunArchivePurgeOnceWithFileTimeout(ctx, st, deleter, now, defaultArchivePurgeFileTimeout)
}

// RunArchivePurgeOnceWithFileTimeout performs one bounded outbox dispatch. It
// exists so callers with a scheduler deadline can set the per-File limit;
// production workers use defaultArchivePurgeFileTimeout.
func RunArchivePurgeOnceWithFileTimeout(ctx context.Context, st *store.StoryStore, deleter FileDeleter, now time.Time, fileTimeout time.Duration) (int64, error) {
	if st == nil || st.Pool == nil {
		return 0, nil
	}
	if fileTimeout <= 0 {
		fileTimeout = defaultArchivePurgeFileTimeout
	}
	batch, err := st.StageArchivePurgeBatch(ctx, 100)
	if err != nil {
		return 0, err
	}
	if deleter == nil {
		return batch.Stories, nil
	}
	// Lease only the operation this tick can attempt. Pre-leasing a batch would
	// leave later rows fenced behind a timed-out File call and risk stale-token
	// delivery after their leases expire.
	ops, err := st.ClaimMediaDeletion(ctx, 1, time.Minute)
	if err != nil {
		return 0, err
	}
	for _, op := range ops {
		if err := deleteFileWithinDeadline(ctx, deleter, op.MediaFileID.String(), fileTimeout); err != nil {
			// A File implementation can ignore cancellation. The dispatcher must
			// still return at its deadline and release the lease for a later tick.
			if _, failErr := st.FailMediaDeletion(context.WithoutCancel(ctx), op.OperationID, op.LeaseToken, err); failErr != nil {
				return 0, failErr
			}
			continue
		}
		_, err := st.CompleteMediaDeletion(ctx, op.OperationID, op.LeaseToken)
		if err != nil {
			return 0, err
		}
	}
	return batch.Stories, nil
}

func deleteFileWithinDeadline(ctx context.Context, deleter FileDeleter, fileID string, timeout time.Duration) error {
	fileCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	select {
	case <-fileCtx.Done():
		return fileCtx.Err()
	default:
	}
	gate := archivePurgeFileCallGateFor(deleter)
	select {
	case gate <- struct{}{}:
	case <-fileCtx.Done():
		return fileCtx.Err()
	default:
		// A non-cooperative File implementation still owns the only bounded
		// in-flight call. Requeue later work instead of leaking a goroutine per
		// operation or blocking the scheduler behind it.
		return errArchivePurgeFileBusy
	}
	result := make(chan error, 1)
	go func() {
		defer func() { <-gate }()
		result <- deleter.DeleteFile(fileCtx, fileID)
	}()
	select {
	case err := <-result:
		return err
	case <-fileCtx.Done():
		return fileCtx.Err()
	}
}

type archivePurgeFileDeleterKey struct {
	typ reflect.Type
	ptr uintptr
}

func archivePurgeFileCallGateFor(deleter FileDeleter) chan struct{} {
	value := reflect.ValueOf(deleter)
	var key any
	if value.IsValid() && value.Kind() == reflect.Ptr && !value.IsNil() {
		key = archivePurgeFileDeleterKey{typ: value.Type(), ptr: value.Pointer()}
	} else {
		// FileDeleter implementations are normally pointers. A non-pointer
		// implementation still receives a bounded gate per concrete type.
		key = reflect.TypeOf(deleter)
	}
	gate, _ := archivePurgeFileCallGates.LoadOrStore(key, make(chan struct{}, 1))
	return gate.(chan struct{})
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

// StartArchivePurgeWorker stages retention cleanup daily and dispatches durable
// media deletion work immediately at startup and on a short retry interval.
func StartArchivePurgeWorker(ctx context.Context, st *store.StoryStore, deleter FileDeleter, logger *slog.Logger) {
	if st == nil {
		return
	}
	go func() {
		purgeTicker := time.NewTicker(24 * time.Hour)
		dispatchTicker := time.NewTicker(time.Minute)
		defer purgeTicker.Stop()
		defer dispatchTicker.Stop()
		run := func() {
			n, err := RunArchivePurgeOnce(ctx, st, deleter, time.Now().UTC())
			if err != nil && logger != nil {
				logger.Error("story archive purge", slog.String("error", err.Error()))
			} else if n > 0 && logger != nil {
				logger.Info("story archive purge", slog.Int64("purged", n))
			}
		}
		run()
		for {
			select {
			case <-ctx.Done():
				return
			case <-purgeTicker.C:
				run()
			case <-dispatchTicker.C:
				// RunArchivePurgeOnce also claims expired leases; staging is bounded.
				run()
			}
		}
	}()
}
