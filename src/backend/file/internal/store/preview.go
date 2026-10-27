package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/filepreview"
)

// PreviewSnapshotTx uses the same serializer for event and exact-reference RPC.
// The caller owns a File row lock; expiry is a transition, never a clock-made revision.
func PreviewSnapshotTx(ctx context.Context, tx pgx.Tx, row FileRow) (*eventsv1.FilePreviewSnapshot, error) {
	if row.IsE2E || row.UploadIntent == "music" {
		return nil, errors.New("preview unsupported")
	}
	var revision uint64
	if err := tx.QueryRow(ctx, `SELECT preview_metadata_revision FROM files WHERE id=$1`, row.ID).Scan(&revision); err != nil {
		return nil, err
	}
	s := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 1, FileId: row.ID.String(), MetadataRevision: revision, Status: row.Status}
	if row.UploadIntent == "gif" {
		s.ProtocolVersion = 2
	}
	if row.Status == "ready" {
		s.FileType, s.UploadIntent, s.SizeBytes, s.ScanResult = row.FileType, row.UploadIntent, row.SizeBytes, row.ScanResult
		s.Width, s.Height, s.DurationSeconds = row.Width, row.Height, row.DurationSeconds
		s.HasThumbnail = row.ThumbnailR2Key != nil && *row.ThumbnailR2Key != ""
		s.HasConverted = row.ConvertedR2Key != nil && *row.ConvertedR2Key != ""
	}
	if err := filepreview.Validate(s); err != nil {
		return nil, err
	}
	return s, nil
}

func recordPreviewChange(ctx context.Context, tx pgx.Tx, before, after FileRow) error {
	if after.IsE2E || after.UploadIntent == "music" {
		return nil
	}
	old, err := PreviewSnapshotTx(ctx, tx, before)
	if err != nil && after.Status != "deleted" && after.Status != "expired" && after.Status != "failed" {
		return err
	}
	next, err := PreviewSnapshotTx(ctx, tx, after)
	if err != nil {
		return err
	}
	if proto.Equal(old, next) {
		return nil
	}
	if filepreview.Terminal(before.Status) {
		return errors.New("terminal file preview cannot change")
	}
	if err := tx.QueryRow(ctx, `UPDATE files SET preview_metadata_revision=preview_metadata_revision+1 WHERE id=$1 RETURNING preview_metadata_revision`, after.ID).Scan(&next.MetadataRevision); err != nil {
		return err
	}
	e := &eventsv1.FileStreamEvent{EventId: uuid.NewString(), OccurredAt: timestamppb.Now(), Payload: &eventsv1.FileStreamEvent_FileProcessed{FileProcessed: &eventsv1.FileProcessed{FileId: next.FileId, Status: next.Status, Preview: next}}}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(e)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO file_preview_event_outbox(event_id,file_id,metadata_revision,payload) VALUES($1,$2,$3,$4)`, e.EventId, after.ID, int64(next.MetadataRevision), raw)
	return err
}

func (s *FilesStore) withPreviewMutation(ctx context.Context, id uuid.UUID, mutate func(pgx.Tx) (FileRow, error)) (FileRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return FileRow{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	before, err := getFileByID(ctx, tx, id, " FOR NO KEY UPDATE")
	if err != nil {
		return FileRow{}, err
	}
	after, err := mutate(tx)
	if err != nil {
		return FileRow{}, err
	}
	if err = recordPreviewChange(ctx, tx, before, after); err != nil {
		return FileRow{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return FileRow{}, err
	}
	return after, nil
}

func (s *FilesStore) DispatchPreviewEvent(ctx context.Context, publish func(context.Context, []byte) error) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT event_id,payload FROM file_preview_event_outbox WHERE published_at IS NULL ORDER BY file_id,metadata_revision FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = publish(ctx, payload); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE file_preview_event_outbox SET published_at=clock_timestamp() WHERE event_id=$1`, id); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
