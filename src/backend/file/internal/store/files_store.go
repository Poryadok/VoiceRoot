package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	Width             *int32
	Height            *int32
	DurationSeconds   *int32
	ThumbnailR2Key    *string
	ConvertedR2Key    *string
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
          r2_key, status, file_type, width, height, duration_seconds,
          thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
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
		&row.Width,
		&row.Height,
		&row.DurationSeconds,
		&row.ThumbnailR2Key,
		&row.ConvertedR2Key,
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
       r2_key, status, file_type, width, height, duration_seconds,
       thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
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
		&row.Width,
		&row.Height,
		&row.DurationSeconds,
		&row.ThumbnailR2Key,
		&row.ConvertedR2Key,
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

func (s *FilesStore) ConfirmUpload(ctx context.Context, id uuid.UUID, sha256Hash string) (FileRow, error) {
	var row FileRow
	err := s.Pool.QueryRow(ctx, `
UPDATE files
SET sha256_hash = $2,
    status = 'ready',
    scan_result = CASE WHEN scan_result = 'pending' THEN 'skipped' ELSE scan_result END,
    updated_at = now()
WHERE id = $1
RETURNING id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
          r2_key, status, file_type, width, height, duration_seconds,
          thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
          scan_result, created_at, updated_at
`, id, sha256Hash).Scan(
		&row.ID,
		&row.UploaderProfileID,
		&row.OriginalName,
		&row.MimeType,
		&row.SizeBytes,
		&row.SHA256Hash,
		&row.R2Key,
		&row.Status,
		&row.FileType,
		&row.Width,
		&row.Height,
		&row.DurationSeconds,
		&row.ThumbnailR2Key,
		&row.ConvertedR2Key,
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

func (s *FilesStore) ApplyImageProcessing(ctx context.Context, id uuid.UUID, convertedKey, thumbnailKey string, width, height int32) (FileRow, error) {
	var row FileRow
	err := s.Pool.QueryRow(ctx, `
UPDATE files
SET converted_r2_key = $2,
    thumbnail_r2_key = $3,
    width = $4,
    height = $5,
    updated_at = now()
WHERE id = $1
RETURNING id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
          r2_key, status, file_type, width, height, duration_seconds,
          thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
          scan_result, created_at, updated_at
`, id, convertedKey, thumbnailKey, width, height).Scan(
		&row.ID,
		&row.UploaderProfileID,
		&row.OriginalName,
		&row.MimeType,
		&row.SizeBytes,
		&row.SHA256Hash,
		&row.R2Key,
		&row.Status,
		&row.FileType,
		&row.Width,
		&row.Height,
		&row.DurationSeconds,
		&row.ThumbnailR2Key,
		&row.ConvertedR2Key,
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

func (s *FilesStore) ApplyScanResult(ctx context.Context, id uuid.UUID, status, scanResult string) (FileRow, error) {
	var row FileRow
	err := s.Pool.QueryRow(ctx, `
UPDATE files
SET status = $2,
    scan_result = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
          r2_key, status, file_type, width, height, duration_seconds,
          thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
          scan_result, created_at, updated_at
`, id, status, scanResult).Scan(
		&row.ID,
		&row.UploaderProfileID,
		&row.OriginalName,
		&row.MimeType,
		&row.SizeBytes,
		&row.SHA256Hash,
		&row.R2Key,
		&row.Status,
		&row.FileType,
		&row.Width,
		&row.Height,
		&row.DurationSeconds,
		&row.ThumbnailR2Key,
		&row.ConvertedR2Key,
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

func (s *FilesStore) GetFilesByIDs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]FileRow, error) {
	rows, err := s.Pool.Query(ctx, `
SELECT id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
       r2_key, status, file_type, width, height, duration_seconds,
       thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
       scan_result, created_at, updated_at
FROM files
WHERE id = ANY($1)
`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[uuid.UUID]FileRow{}
	for rows.Next() {
		var row FileRow
		if err := rows.Scan(
			&row.ID,
			&row.UploaderProfileID,
			&row.OriginalName,
			&row.MimeType,
			&row.SizeBytes,
			&row.SHA256Hash,
			&row.R2Key,
			&row.Status,
			&row.FileType,
			&row.Width,
			&row.Height,
			&row.DurationSeconds,
			&row.ThumbnailR2Key,
			&row.ConvertedR2Key,
			&row.ChatID,
			&row.ChatType,
			&row.IsE2E,
			&row.ExpiresAt,
			&row.ScanResult,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out[row.ID] = row
	}
	return out, rows.Err()
}

func (s *FilesStore) ListFilesForProfile(ctx context.Context, profileID uuid.UUID, limit int32) ([]FileRow, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.Pool.Query(ctx, `
SELECT id, uploader_profile_id, original_name, mime_type, size_bytes, sha256_hash,
       r2_key, status, file_type, width, height, duration_seconds,
       thumbnail_r2_key, converted_r2_key, chat_id, chat_type, is_e2e, expires_at,
       scan_result, created_at, updated_at
FROM files
WHERE uploader_profile_id = $1
  AND status <> 'deleted'
ORDER BY created_at DESC, id DESC
LIMIT $2
`, profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []FileRow
	for rows.Next() {
		var row FileRow
		if err := rows.Scan(
			&row.ID,
			&row.UploaderProfileID,
			&row.OriginalName,
			&row.MimeType,
			&row.SizeBytes,
			&row.SHA256Hash,
			&row.R2Key,
			&row.Status,
			&row.FileType,
			&row.Width,
			&row.Height,
			&row.DurationSeconds,
			&row.ThumbnailR2Key,
			&row.ConvertedR2Key,
			&row.ChatID,
			&row.ChatType,
			&row.IsE2E,
			&row.ExpiresAt,
			&row.ScanResult,
			&row.CreatedAt,
			&row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

func (s *FilesStore) MarkDeleted(ctx context.Context, id uuid.UUID) error {
	tag, err := s.Pool.Exec(ctx, `
UPDATE files
SET status = 'deleted',
    updated_at = now()
WHERE id = $1
`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *FilesStore) BytesUsedByProfile(ctx context.Context, profileID uuid.UUID) (int64, error) {
	var used int64
	err := s.Pool.QueryRow(ctx, `
SELECT COALESCE(SUM(size_bytes), 0)
FROM files
WHERE uploader_profile_id = $1
  AND status NOT IN ('deleted', 'expired')
`, profileID).Scan(&used)
	return used, err
}
