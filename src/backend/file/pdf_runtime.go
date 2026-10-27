package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"time"

	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/pdfproc"
	"voice/backend/file/internal/store"
)

// startPDFRuntime activates the reviewed worker only when its physical-write
// custody and native renderer are installed. The caller owns its lifetime.
func startPDFRuntime(ctx context.Context, files *store.FilesStore, objects jobs.PDFStorage, logger *slog.Logger) error {
	if files == nil || files.Pool == nil || objects == nil {
		return errors.New("PDF worker requires persistence and object storage")
	}
	binary, err := exec.LookPath("pdftoppm")
	if err != nil {
		return fmt.Errorf("PDF renderer unavailable: %w", err)
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var ready bool
	err = files.Pool.QueryRow(probeCtx, `SELECT
 EXISTS(SELECT 1 FROM pg_attribute WHERE attrelid=to_regclass('file_catalog_writes') AND attname='owner_kind' AND attnotnull AND NOT attisdropped)
 AND EXISTS(SELECT 1 FROM pg_constraint WHERE conrelid=to_regclass('file_catalog_writes') AND conname='file_catalog_writes_owner_kind_check' AND contype='c' AND convalidated)
 AND EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=to_regclass('file_blobs') AND tgname='file_catalog_guard_blob_gc' AND tgenabled IN ('O','A') AND NOT tgisinternal)
 AND to_regclass('file_preview_event_outbox') IS NOT NULL`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("PDF worker schema readiness: %w", err)
	}
	if !ready {
		return errors.New("PDF worker requires migrations 000010 and 000011 with active tracked-write GC guard")
	}
	jobs.StartPDFWorker(ctx, files, objects, pdfproc.Renderer{Binary: binary}, time.Second, logger)
	if logger != nil {
		logger.Info("PDF thumbnail worker enabled")
	}
	return nil
}
