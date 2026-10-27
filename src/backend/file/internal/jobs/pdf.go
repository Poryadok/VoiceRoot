package jobs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"voice/backend/file/internal/pdfproc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
)

type PDFStorage interface {
	ReadObject(context.Context, string, int64) ([]byte, error)
	PutTrackedObject(context.Context, r2file.TrackedPut, []byte) error
}
type PDFRenderer interface {
	Render(context.Context, []byte) ([]byte, error)
}

// RunPDFThumbnailOnce processes one reserved plaintext PDF. The original is
// deliberately retained: document downloads preserve their admitted bytes.
func RunPDFThumbnailOnce(ctx context.Context, files *store.FilesStore, objects PDFStorage, renderer PDFRenderer) (bool, error) {
	if files == nil || files.Pool == nil || objects == nil || renderer == nil {
		return false, errors.New("PDF processing not configured")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	return files.ProcessPDFThumbnailOnce(ctx, func(ctx context.Context, row store.FileRow) ([]byte, bool, error) {
		if row.SizeBytes <= 0 || row.SizeBytes > pdfproc.MaxBytes {
			return nil, true, pdfproc.ErrInvalidPDF
		}
		raw, err := objects.ReadObject(ctx, row.R2Key, row.SizeBytes)
		if err != nil {
			return nil, false, err
		}
		if int64(len(raw)) != row.SizeBytes {
			return nil, true, pdfproc.ErrInvalidPDF
		}
		digest := sha256.Sum256(raw)
		if row.SHA256Hash == nil || !strings.EqualFold(hex.EncodeToString(digest[:]), *row.SHA256Hash) {
			return nil, true, pdfproc.ErrInvalidPDF
		}
		thumb, err := renderer.Render(ctx, raw)
		if err != nil {
			return nil, errors.Is(err, pdfproc.ErrInvalidPDF), err
		}
		if len(thumb) == 0 || len(thumb) > pdfproc.MaxThumbnailBytes {
			return nil, true, pdfproc.ErrInvalidPDF
		}
		return thumb, false, nil
	}, func(ctx context.Context, intent store.TrackedWriteIntent, thumb []byte) error {
		return objects.PutTrackedObject(ctx, r2file.TrackedPut{Key: intent.Key, Kind: intent.Kind, ContentType: "image/png", ExpectedSHA256: intent.ExpectedSHA256, ExpectedSize: intent.ExpectedSize}, thumb)
	})
}

func StartPDFWorker(ctx context.Context, files *store.FilesStore, objects PDFStorage, renderer PDFRenderer, interval time.Duration, logger *slog.Logger) {
	if files == nil || files.Pool == nil || objects == nil || renderer == nil {
		return
	}
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		var lastSettlementReport time.Time
		for {
			_, err := RunPDFThumbnailOnce(ctx, files, objects, renderer)
			if err != nil && ctx.Err() == nil && logger != nil {
				logger.Error("PDF thumbnail processing failed", slog.String("error", err.Error()))
			}
			if logger != nil && ctx.Err() == nil && time.Since(lastSettlementReport) >= time.Minute {
				lastSettlementReport = time.Now()
				var count int64
				observationCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := files.Pool.QueryRow(observationCtx, `SELECT count(*) FROM file_catalog_writes WHERE owner_kind='pdf_thumbnail' AND state IN ('STARTED','UNSETTLED')`).Scan(&count)
				cancel()
				if err != nil {
					logger.Error("PDF write settlement observation failed", slog.String("error", err.Error()))
				} else if count > 0 {
					logger.Warn("PDF writes await physical settlement", slog.Int64("pending", count))
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}
