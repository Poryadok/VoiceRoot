package jobs

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"voice/backend/file/internal/pdfproc"
	"voice/backend/file/internal/store"
)

type PDFStorage interface {
	ReadObject(context.Context, string, int64) ([]byte, error)
	PutObject(context.Context, string, string, []byte) error
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
	return files.ProcessPDFThumbnailOnce(ctx, func(ctx context.Context, row store.FileRow) (bool, error) {
		if row.SizeBytes <= 0 || row.SizeBytes > pdfproc.MaxBytes {
			return true, pdfproc.ErrInvalidPDF
		}
		raw, err := objects.ReadObject(ctx, row.R2Key, row.SizeBytes)
		if err != nil {
			return false, err
		}
		if int64(len(raw)) != row.SizeBytes {
			return true, pdfproc.ErrInvalidPDF
		}
		thumb, err := renderer.Render(ctx, raw)
		if err != nil {
			return errors.Is(err, pdfproc.ErrInvalidPDF), err
		}
		if len(thumb) == 0 || len(thumb) > pdfproc.MaxThumbnailBytes {
			return true, pdfproc.ErrInvalidPDF
		}
		err = objects.PutObject(ctx, *row.ThumbnailR2Key, "image/png", thumb)
		return false, err
	})
}

func StartPDFWorker(ctx context.Context, files *store.FilesStore, objects PDFStorage, renderer PDFRenderer, interval time.Duration, logger *slog.Logger) {
	if interval <= 0 {
		interval = time.Second
	}
	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			_, err := RunPDFThumbnailOnce(ctx, files, objects, renderer)
			if err != nil && ctx.Err() == nil && logger != nil {
				logger.Error("PDF thumbnail processing failed", slog.String("error", err.Error()))
			}
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
		}
	}()
}
