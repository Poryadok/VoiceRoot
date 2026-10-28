package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"time"
	"voice/backend/file/internal/audioproc"
	"voice/backend/file/internal/fileevents"
)

func (s *FilesStore) markUnavailableWithMusicEvent(ctx context.Context, id uuid.UUID, status string) error {
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	row, err := getFileByID(ctx, tx, id, "FOR NO KEY UPDATE")
	if err != nil {
		return err
	}
	if err = markUnavailableWithMusicEventTx(ctx, tx, row, status); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Caller owns the file lock; legacy deletion uses NO KEY UPDATE to avoid
// deadlocking with a blob-locked reference insertion's FK KEY SHARE.
func markUnavailableWithMusicEventTx(ctx context.Context, tx pgx.Tx, row FileRow, status string) error {
	id := row.ID
	if status == "expired" && row.Status != "ready" {
		return pgx.ErrNoRows
	}
	if status == "deleted" || status == "expired" {
		if err := queueGIFOriginalCleanupTx(ctx, tx, row); err != nil {
			return err
		}
	}
	if row.Status == status {
		return nil
	}
	music := row.UploadIntent == "music" && row.FileType == "audio" && !row.IsE2E
	if _, err := tx.Exec(ctx, `UPDATE files SET status=$2,updated_at=now(),
 music_metadata_revision=music_metadata_revision+CASE WHEN $3 THEN 1 ELSE 0 END,
 music_title=CASE WHEN $3 THEN NULL ELSE music_title END,
 music_artist=CASE WHEN $3 THEN NULL ELSE music_artist END,
 music_album=CASE WHEN $3 THEN NULL ELSE music_album END
 WHERE id=$1`, id, status, music); err != nil {
		return err
	}
	if music && row.ChatID != nil {
		event, eventErr := fileevents.NewMusicTombstone(id.String(), row.ChatID.String(), uint64(row.MusicMetadataRevision+1), status)
		if eventErr != nil {
			return eventErr
		}
		body, eventErr := proto.Marshal(event)
		if eventErr != nil {
			return eventErr
		}
		if _, err := tx.Exec(ctx, `INSERT INTO file_music_event_outbox(event_id,file_id,metadata_revision,payload) VALUES($1,$2,$3,$4)`, event.GetEventId(), id, row.MusicMetadataRevision+1, body); err != nil {
			return err
		}
	}
	after := row
	after.Status = status
	if status == "expired" {
		if err := recordExpiryEvent(ctx, tx, row); err != nil {
			return err
		}
	}
	return recordPreviewChange(ctx, tx, row, after)
}

// ApplyAudioProcessing commits metadata and its processing event together. No clock value serves as
// a revision; retries consume the same durable snapshot.
func (s *FilesStore) ApplyAudioProcessing(ctx context.Context, id uuid.UUID, title, artist, album string, duration int32) (FileRow, error) {
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return FileRow{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	tag, err := tx.Exec(ctx, `UPDATE files SET status='ready',music_title=NULLIF($2,''),music_artist=NULLIF($3,''),music_album=NULLIF($4,''),duration_seconds=$5,music_metadata_revision=music_metadata_revision+1,updated_at=now()
WHERE id=$1 AND status='processing' AND upload_intent='music' AND file_type='audio' AND NOT is_e2e AND scan_result IN ('clean','skipped') AND size_bytes<=10485760`, id, title, artist, album, duration)
	if err != nil {
		return FileRow{}, err
	}
	if tag.RowsAffected() != 1 {
		return FileRow{}, pgx.ErrNoRows
	}
	row, err := getFileByID(ctx, tx, id, "")
	if err != nil {
		return FileRow{}, err
	}
	if row.ChatID != nil {
		event, eventErr := fileevents.NewMusicProcessed(id.String(), row.ChatID.String(), uint64(row.MusicMetadataRevision), row.SizeBytes, row.ScanResult, audioproc.Metadata{Title: title, Artist: artist, Album: album, DurationSeconds: duration})
		if eventErr != nil {
			return FileRow{}, eventErr
		}
		payload, eventErr := proto.Marshal(event)
		if eventErr != nil {
			return FileRow{}, eventErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO file_music_event_outbox(event_id,file_id,metadata_revision,payload) VALUES($1,$2,$3,$4)`, event.GetEventId(), id, row.MusicMetadataRevision, payload); err != nil {
			return FileRow{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return FileRow{}, err
	}
	return row, nil
}

// DispatchMusicEvent holds only one SKIP LOCKED row through a bounded PubAck.
// A crash after PubAck can redeliver; revision checks at Messaging make that
// harmless. Failure never marks publication successful.
func (s *FilesStore) DispatchMusicEvent(ctx context.Context, publish func(context.Context, []byte) error) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := BeginInventoryMutation(ctx, s.Pool, true)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT event_id,payload FROM file_music_event_outbox WHERE published_at IS NULL ORDER BY file_id,metadata_revision FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &payload)
	if err == pgx.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = publish(ctx, payload); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE file_music_event_outbox SET published_at=now() WHERE event_id=$1`, id); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
