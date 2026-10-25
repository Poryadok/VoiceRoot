package store

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/messageevents"
)

// LegacyPreviewRows pages the authoritative owner rows, never an event's JSON.
// Each completed pass wraps to catch concurrent inserts below the watermark.
func (s *MessagesStore) LegacyPreviewRows(ctx context.Context) ([]MessageRow, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var after uuid.UUID
	if err = tx.QueryRow(ctx, `SELECT after_id FROM messaging_file_preview_reconciliation WHERE singleton FOR UPDATE SKIP LOCKED`).Scan(&after); errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, messageSelectSQL+` FROM messages WHERE id>$1 AND deleted_at IS NULL AND NOT is_e2e AND COALESCE(content_type,'')<>'music' AND attachments<>'[]'::jsonb AND file_preview_bindings='[]'::jsonb AND NOT EXISTS(SELECT 1 FROM messaging_file_preview_errors e WHERE e.message_id=messages.id AND e.next_attempt_at>clock_timestamp()) ORDER BY id LIMIT 100`, after)
	if err != nil {
		return nil, err
	}
	var result []MessageRow
	for rows.Next() {
		row, e := scanMessageRow(rows)
		if e != nil {
			rows.Close()
			return nil, e
		}
		result = append(result, *row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if len(result) == 100 {
		_, err = tx.Exec(ctx, `UPDATE messaging_file_preview_reconciliation SET after_id=$1,updated_at=clock_timestamp() WHERE singleton`, result[len(result)-1].ID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE messaging_file_preview_reconciliation SET after_id='00000000-0000-0000-0000-000000000000',pass=pass+1,updated_at=clock_timestamp() WHERE singleton`)
	}
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *MessagesStore) RecordLegacyPreviewError(ctx context.Context, id uuid.UUID, reason string) error {
	// Categories only: errors never retain credentials, URLs or wire payloads.
	_, err := s.Pool.Exec(ctx, `INSERT INTO messaging_file_preview_errors(message_id,reason,next_attempt_at) VALUES($1,$2,clock_timestamp()+interval '1 minute') ON CONFLICT(message_id) DO UPDATE SET attempts=messaging_file_preview_errors.attempts+1,reason=EXCLUDED.reason,next_attempt_at=EXCLUDED.next_attempt_at,updated_at=clock_timestamp()`, id, reason)
	return err
}

// PrepareLegacyPreviewAcquisition retains the exact owner operation before RPC.
// The ordinary abandoned-intent recovery releases an acquisition without a live row.
type legacyPreviewDB interface {
	Begin(context.Context) (pgx.Tx, error)
}

func (s *MessagesStore) prepareLegacyPreviewAcquisition(ctx context.Context, db legacyPreviewDB, row MessageRow, space *uuid.UUID, request *filev1.AcquireFileReferencesRequest) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockMessageMutation(ctx, tx, row.ChatID, space, &row.ID); err != nil {
		return err
	}
	if err = requireLiveMessage(ctx, tx, row.ChatID, row.ID); err != nil {
		return err
	}
	// Preserve an existing exact canonical send intent, if any. Otherwise this
	// persisted row and durable deterministic operation own the reference repair.
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(request)
	if err != nil {
		return err
	}
	op := uuid.NewSHA1(row.ID, []byte("voice.message.preview.backfill.v1"))
	_, err = tx.Exec(ctx, `INSERT INTO messaging_attachment_send_intents(operation_id,message_id,chat_id,scope_space_id,request_sha256,references_bytes,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(message_id) DO NOTHING`, op, row.ID, row.ChatID, space, lifecyclePreviewHash(raw), raw, time.Now().Add(time.Hour))
	if err != nil {
		return err
	}
	var savedRaw []byte
	var savedOp uuid.UUID
	var state string
	if err = tx.QueryRow(ctx, `SELECT operation_id,references_bytes,state FROM messaging_attachment_send_intents WHERE message_id=$1`, row.ID).Scan(&savedOp, &savedRaw, &state); err != nil {
		return err
	}
	// An unrelated completed send whose reference vanished needs explicit owner
	// repair. Never restart its operation behind a concurrent keyed caller.
	if savedOp != op || state != "PENDING" {
		return ErrAttachmentIntentConflict
	}
	var saved filev1.AcquireFileReferencesRequest
	if err = proto.Unmarshal(savedRaw, &saved); err != nil {
		return err
	}
	if saved.ProducerId != request.ProducerId || len(saved.References) != len(request.References) {
		return ErrAttachmentIntentConflict
	}
	for i, ref := range request.References {
		if !proto.Equal(ref, saved.References[i]) {
			return ErrAttachmentIntentConflict
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE messaging_attachment_send_intents SET expires_at=clock_timestamp()+interval '1 hour' WHERE operation_id=$1 AND state='PENDING'`, op); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func lifecyclePreviewHash(raw []byte) []byte { sum := sha256.Sum256(raw); return sum[:] }

func (s *MessagesStore) BindLegacyFilePreviews(ctx context.Context, row MessageRow, space *uuid.UUID) error {
	return s.bindLegacyFilePreviews(ctx, s.Pool, row, space)
}

// RepairLegacyFilePreviews shares the exact intent session lock with abandoned
// acquisition cleanup. Its own connection carries both transactions so pool
// waiters cannot starve the lock holder during the bounded external call.
func (s *MessagesStore) RepairLegacyFilePreviews(ctx context.Context, row MessageRow, space *uuid.UUID, request *filev1.AcquireFileReferencesRequest, prepare func(context.Context, *MessageRow, *filev1.AcquireFileReferencesRequest) error) error {
	op := uuid.NewSHA1(row.ID, []byte("voice.message.preview.backfill.v1"))
	conn, unlock, err := attachmentIntentLock(ctx, s.Pool, op)
	if err != nil {
		return err
	}
	defer unlock()
	if err = s.prepareLegacyPreviewAcquisition(ctx, conn, row, space, request); err != nil {
		return err
	}
	if err = prepare(ctx, &row, request); err != nil {
		return err
	}
	return s.bindLegacyFilePreviews(ctx, conn, row, space)
}

func (s *MessagesStore) bindLegacyFilePreviews(ctx context.Context, db legacyPreviewDB, row MessageRow, space *uuid.UUID) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockMessageMutation(ctx, tx, row.ChatID, space, &row.ID); err != nil {
		return err
	}
	var raw, bindings string
	var encrypted, ghost bool
	err = tx.QueryRow(ctx, `SELECT attachments::text,file_preview_bindings::text,is_e2e,ghost_only FROM messages WHERE id=$1 AND chat_id=$2 AND deleted_at IS NULL FOR UPDATE`, row.ID, row.ChatID).Scan(&raw, &bindings, &encrypted, &ghost)
	if err != nil {
		return err
	}
	if encrypted {
		return errors.New("encrypted legacy preview denied")
	}
	if bindings != "[]" {
		return tx.Commit(ctx)
	}
	row.AttachmentsJSON = raw
	// Legacy unavailable files must become unavailable projections too. New send
	// insertion remains ready-only; here the row already exists and is authorized.
	ordered := append([]FilePreviewBinding(nil), row.FilePreviewBindings...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].FileID < ordered[j].FileID })
	current := map[string]*eventsv1.FilePreviewSnapshot{}
	for _, b := range ordered {
		if err = validatePreviewBinding(row, b); err != nil {
			return err
		}
		if !previewBindingScope(b, space) {
			return errors.New("legacy preview scope mismatch")
		}
		snapshot, e := persistPreviewTx(ctx, tx, b.Snapshot)
		if e != nil {
			return e
		}
		current[b.FileID] = snapshot
	}
	for i := range row.FilePreviewBindings {
		b := &row.FilePreviewBindings[i]
		row.AttachmentsJSON, err = applyPreviewAttachment(row.AttachmentsJSON, *b, current[b.FileID])
		if err != nil {
			return err
		}
		b.Snapshot = current[b.FileID]
	}
	encoded, err := json.Marshal(row.FilePreviewBindings)
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE messages SET attachments=$2::jsonb,file_preview_bindings=$3::jsonb WHERE id=$1`, row.ID, row.AttachmentsJSON, string(encoded)); err != nil {
		return err
	}
	if raw != row.AttachmentsJSON && !ghost {
		event, e := messageevents.NewMessageEditedOutbox(row.ID.String(), row.ChatID.String(), false)
		if e != nil {
			return e
		}
		if err = enqueueMessageEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	if _, err = tx.Exec(ctx, `DELETE FROM messaging_file_preview_errors WHERE message_id=$1`, row.ID); err != nil {
		return err
	}
	op := uuid.NewSHA1(row.ID, []byte("voice.message.preview.backfill.v1"))
	if _, err = tx.Exec(ctx, `UPDATE messaging_attachment_send_intents SET state='COMMITTED',completed_at=clock_timestamp() WHERE operation_id=$1 AND state='PENDING'`, op); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
