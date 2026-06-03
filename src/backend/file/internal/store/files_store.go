package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FileRow struct {
	ID                uuid.UUID
	UploaderProfileID uuid.UUID
	OriginalName      string
	MimeType          string
	SizeBytes         int64
	SHA256Hash        *string
	R2Key             string
	Status            string
	FileType          string
	ChatID            *uuid.UUID
	ChatType          *string
	IsE2E             bool
	ExpiresAt         *time.Time
	ScanResult        string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type FilesStore struct {
	Pool *pgxpool.Pool
}

func NewFilesStore(pool *pgxpool.Pool) *FilesStore {
	return &FilesStore{Pool: pool}
}

func (s *FilesStore) InsertPendingFile(ctx context.Context, row FileRow) (FileRow, error) {
	err := s.Pool.QueryRow(ctx, `
INSERT INTO files (
  id,
  uploader_profile_id,
  original_name,
  mime_type,
  size_bytes,
  r2_key,
  status,
  file_type,
  chat_id,
  chat_type,
  is_e2e,
  scan_result
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
          r2_key, status, file_type, chat_id, chat_type, is_e2e, expires_at,
          scan_result, created_at, updated_at
`, row.ID, row.UploaderProfileID, row.OriginalName, row.MimeType, row.SizeBytes, row.R2Key,
		row.Status, row.FileType, row.ChatID, row.ChatType, row.IsE2E, row.ScanResult).Scan(
		&row.ID,
		&row.UploaderProfileID,
		&row.OriginalName,
		&row.MimeType,
		&row.SizeBytes,
		&row.SHA256Hash,
		&row.R2Key,
		&row.Status,
		&row.FileType,
		&row.ChatID,
		&row.ChatType,
		&row.IsE2E,
		&row.ExpiresAt,
		&row.ScanResult,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	return row, err
}

func (s *FilesStore) GetFileByID(ctx context.Context, id uuid.UUID) (FileRow, error) {
	var row FileRow
	err := s.Pool.QueryRow(ctx, `
SELECT id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
       r2_key, status, file_type, chat_id, chat_type, is_e2e, expires_at,
       scan_result, created_at, updated_at
FROM files
WHERE id = $1
`, id).Scan(
		&row.ID,
		&row.UploaderProfileID,
		&row.OriginalName,
		&row.MimeType,
		&row.SizeBytes,
		&row.SHA256Hash,
		&row.R2Key,
		&row.Status,
		&row.FileType,
		&row.ChatID,
		&row.ChatType,
		&row.IsE2E,
		&row.ExpiresAt,
		&row.ScanResult,
		&row.CreatedAt,
		&row.UpdatedAt,
	)
	return row, err
}
