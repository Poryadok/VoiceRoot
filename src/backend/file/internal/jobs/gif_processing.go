package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"voice/backend/file/internal/gifproc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

type gifObjectStorage interface {
	r2file.ObjectReader
	r2file.ObjectDeleter
	PutObject(context.Context, string, string, []byte) error
}

type gifTrackedObjectStorage interface {
	PutTrackedObject(context.Context, r2file.TrackedPut, []byte) error
}

// RunGIFProcessingOnce consumes one durable attempt. It never deletes its own
// outputs directly: even an ambiguous DB commit or storage response is reconciled
// using the persisted attempt ownership and cleanup ledger.
func RunGIFProcessingOnce(ctx context.Context, files *store.FilesStore, objects gifObjectStorage, convert func(context.Context, []byte) (gifproc.Output, error)) (bool, error) {
	if files == nil || objects == nil {
		return false, nil
	}
	if reconciled, err := files.ReconcileRetiredGIFWritesOnce(ctx); err != nil || reconciled {
		return reconciled, err
	}
	job, err := files.ClaimGIFJob(ctx)
	if err != nil || job == nil {
		return false, err
	}
	attemptCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	raw, err := objects.ReadObject(attemptCtx, job.Row.R2Key, job.Row.SizeBytes)
	if err != nil {
		_ = files.RetryGIFJob(ctx, job)
		return true, err
	}
	sum := sha256.Sum256(raw)
	if job.Row.SHA256Hash == nil || int64(len(raw)) != job.Row.SizeBytes || hex.EncodeToString(sum[:]) != *job.Row.SHA256Hash {
		_, err = files.FinishGIFJob(ctx, job, nil, nil, 0, 0, 0, true)
		return true, err
	}
	if convert == nil {
		convert = gifproc.Convert
	}
	output, err := convert(attemptCtx, raw)
	if err != nil {
		if errors.Is(err, gifproc.ErrInvalid) || errors.Is(err, gifproc.ErrTooLarge) {
			_, err = files.FinishGIFJob(ctx, job, nil, nil, 0, 0, 0, true)
		} else {
			_ = files.RetryGIFJob(ctx, job)
		}
		return true, err
	}
	if len(output.Full) == 0 || len(output.Thumbnail) == 0 || len(output.Full) > 5*1024*1024 || len(output.Thumbnail) > 5*1024*1024 {
		_, err = files.FinishGIFJob(ctx, job, nil, nil, 0, 0, 0, true)
		return true, err
	}
	full := gifDescriptor(job.FullKey, "video/mp4", job.Row.ID.String()+".mp4", output.Full)
	thumb := gifDescriptor(job.ThumbnailKey, "image/webp", job.Row.ID.String()+"-thumbnail.webp", output.Thumbnail)
	put := func(kind string, r *store.Representation, body []byte) error {
		return objects.PutObject(attemptCtx, r.Key, r.MIMEType, body)
	}
	if job.TrackedWrites {
		writer, ok := objects.(gifTrackedObjectStorage)
		if !ok {
			return true, store.ErrTrackedWrite
		}
		if err = files.PrepareGIFTrackedWrites(attemptCtx, job, full, thumb); err != nil {
			return true, err
		}
		put = func(kind string, r *store.Representation, body []byte) error {
			w, err := files.StartTrackedWrite(attemptCtx, r.Key, job.Token)
			if err != nil {
				return err
			}
			putErr := writer.PutTrackedObject(attemptCtx, r2file.TrackedPut{Key: w.Key, Kind: kind, ContentType: r.MIMEType, ExpectedSHA256: w.ExpectedSHA256, ExpectedSize: w.ExpectedSize}, body)
			// This records physical custody even if deletion, session loss or
			// cancellation has already removed publication authority.
			settleErr := files.SettleTrackedWrite(ctx, w.Key, job.Token, putErr == nil)
			return errors.Join(putErr, settleErr)
		}
	}
	if err = put("gif_full", full, output.Full); err != nil {
		_ = files.RetryGIFJob(ctx, job)
		return true, err
	}
	if err = put("gif_thumbnail", thumb, output.Thumbnail); err != nil {
		_ = files.RetryGIFJob(ctx, job)
		return true, err
	}
	_, err = files.FinishGIFJob(ctx, job, full, thumb, output.Width, output.Height, output.DurationSeconds, false)
	return true, err
}

func gifDescriptor(key, mime, name string, body []byte) *store.Representation {
	sum := sha256.Sum256(body)
	return &store.Representation{Key: key, MIMEType: mime, Filename: name, SizeBytes: int64(len(body)), SHA256: hex.EncodeToString(sum[:]), ProcessorVersion: store.GIFProcessorVersion}
}

// StartGIFProcessingWorker is wired to the ordinary File upload runtime. No
// provider, picker or release-capability flag is required by this API consumer.
func StartGIFProcessingWorker(ctx context.Context, files *store.FilesStore, objects gifObjectStorage, logger *slog.Logger) {
	if files == nil || objects == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			if _, err := RunGIFProcessingOnce(ctx, files, objects, nil); err != nil && logger != nil {
				logger.WarnContext(ctx, "GIF processing recovery pending")
			}
			for i := 0; i < 16; i++ {
				done, err := files.CleanupGIFOnce(ctx, objects)
				if err != nil {
					if logger != nil {
						logger.WarnContext(ctx, "GIF object cleanup pending")
					}
					break
				}
				if !done {
					break
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}
