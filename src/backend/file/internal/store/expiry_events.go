package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	"voice/backend/file/internal/fileevents"
)

func recordExpiryEvent(ctx context.Context, tx pgx.Tx, row FileRow) error {
	var chat *string
	if row.ChatID != nil {
		value := row.ChatID.String()
		chat = &value
	}
	event, err := fileevents.NewExpiryEvent(row.ID.String(), chat)
	if err != nil {
		return err
	}
	payload, err := (proto.MarshalOptions{Deterministic: true}).Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO file_expiry_event_outbox(event_id,file_id,payload) VALUES($1,$2,$3)`, event.EventId, row.ID, payload)
	return err
}

// The row lock leases one pending event through a bounded PubAck. On uncertain
// publication/completion the retained exact envelope may be delivered again.
func (s *FilesStore) DispatchExpiryEvent(ctx context.Context, publish func(context.Context, []byte) error) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id uuid.UUID
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT event_id,payload FROM file_expiry_event_outbox WHERE published_at IS NULL ORDER BY created_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err = publish(ctx, payload); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `UPDATE file_expiry_event_outbox SET published_at=clock_timestamp() WHERE event_id=$1`, id); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
