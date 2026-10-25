package store

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"sort"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/messageevents"
	"voice/backend/pkg/filepreview"
)

// FilePreviewBinding is established only from a protected exact-reference RPC.
// It is private owner state, never accepted from attachment JSON or sent to clients.
type FilePreviewBinding struct {
	Slot      int                           `json:"slot"`
	FileID    string                        `json:"file_id"`
	Kind      string                        `json:"kind"`
	Reference *filev1.FileReferenceKey      `json:"reference"`
	Snapshot  *eventsv1.FilePreviewSnapshot `json:"snapshot"`
}

func previewBytes(s *eventsv1.FilePreviewSnapshot) ([]byte, error) {
	return (proto.MarshalOptions{Deterministic: true}).Marshal(s)
}

func persistPreviewTx(ctx context.Context, tx pgx.Tx, in *eventsv1.FilePreviewSnapshot) (*eventsv1.FilePreviewSnapshot, error) {
	if err := filepreview.Validate(in); err != nil {
		return nil, err
	}
	raw, err := previewBytes(in)
	if err != nil {
		return nil, err
	}
	// The insert serializes absent-row races too; no advisory hash collisions.
	_, err = tx.Exec(ctx, `INSERT INTO messaging_file_preview_snapshots(file_id,metadata_revision,payload,terminal) VALUES($1,$2,$3,$4) ON CONFLICT DO NOTHING`, in.FileId, int64(in.MetadataRevision), raw, filepreview.Terminal(in.Status))
	if err != nil {
		return nil, err
	}
	var oldRaw []byte
	var rev int64
	var terminal bool
	err = tx.QueryRow(ctx, `SELECT metadata_revision,payload,terminal FROM messaging_file_preview_snapshots WHERE file_id=$1 FOR UPDATE`, in.FileId).Scan(&rev, &oldRaw, &terminal)
	if err != nil {
		return nil, err
	}
	if uint64(rev) == in.MetadataRevision && !bytes.Equal(oldRaw, raw) {
		return nil, errors.New("conflicting equal file preview revision")
	}
	if in.MetadataRevision > uint64(rev) {
		if terminal && !filepreview.Terminal(in.Status) {
			return nil, errors.New("terminal file preview resurrection")
		}
		_, err = tx.Exec(ctx, `UPDATE messaging_file_preview_snapshots SET metadata_revision=$2,payload=$3,terminal=$4,updated_at=clock_timestamp() WHERE file_id=$1`, in.FileId, int64(in.MetadataRevision), raw, terminal || filepreview.Terminal(in.Status))
		if err != nil {
			return nil, err
		}
		oldRaw = raw
	}
	var current eventsv1.FilePreviewSnapshot
	if err = proto.Unmarshal(oldRaw, &current); err != nil {
		return nil, err
	}
	return &current, nil
}

// PersistFilePreview retains the fact and durable destination work atomically,
// including an event with no existing destination. Insert reconciliation closes it.
func (s *MessagesStore) PersistFilePreview(ctx context.Context, e *eventsv1.FileProcessed) error {
	if err := filepreview.ValidateEvent(e); err != nil {
		return err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = persistPreviewTx(ctx, tx, e.Preview); err != nil {
		return err
	}
	needle, _ := json.Marshal([]map[string]string{{"file_id": e.FileId}})
	_, err = tx.Exec(ctx, `INSERT INTO messaging_file_preview_pending(file_id,chat_id) SELECT $1,chat_id FROM messages WHERE file_preview_bindings @> $2::jsonb AND deleted_at IS NULL AND NOT is_e2e GROUP BY chat_id ON CONFLICT(file_id,chat_id) DO UPDATE SET updated_at=clock_timestamp()`, e.FileId, string(needle))
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func validatePreviewBinding(row MessageRow, b FilePreviewBinding) error {
	if !filepreview.AttachmentKind(b.Kind) {
		return errors.New("unsupported preview attachment kind")
	}
	if row.IsE2E || b.Slot < 0 || b.Reference == nil || b.Reference.OwnerType != filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE || b.Reference.OwnerId != row.ID.String() || b.Reference.SubresourceId != nil || b.Reference.FileId != b.FileID || b.Snapshot.GetFileId() != b.FileID || b.Kind == "music" {
		return errors.New("invalid private preview binding")
	}
	return filepreview.Validate(b.Snapshot)
}

func previewBindingScope(b FilePreviewBinding, space *uuid.UUID) bool {
	if space == nil {
		return b.Reference != nil && b.Reference.ScopeSpaceId == nil
	}
	return b.Reference != nil && b.Reference.ScopeSpaceId != nil && b.Reference.GetScopeSpaceId() == space.String()
}

func (s *MessagesStore) reconcileFilePreviewInsert(ctx context.Context, tx pgx.Tx, row *MessageRow, space *uuid.UUID) error {
	if len(row.FilePreviewBindings) == 0 {
		return nil
	}
	// Each file lock is taken once in raw UUID order, independent of slot order.
	ordered := append([]FilePreviewBinding(nil), row.FilePreviewBindings...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].FileID < ordered[j].FileID })
	current := map[string]*eventsv1.FilePreviewSnapshot{}
	for _, b := range ordered {
		if !previewBindingScope(b, space) {
			return errors.New("preview admission scope mismatch")
		}
		if err := validatePreviewBinding(*row, b); err != nil {
			return err
		}
		snapshot, err := persistPreviewTx(ctx, tx, b.Snapshot)
		if err != nil {
			return err
		}
		if snapshot.Status != "ready" {
			return errors.New("file preview is no longer ready")
		}
		current[b.FileID] = snapshot
	}
	raw := row.AttachmentsJSON
	for i := range row.FilePreviewBindings {
		b := &row.FilePreviewBindings[i]
		snapshot := current[b.FileID]
		updated, err := applyPreviewAttachment(raw, *b, snapshot)
		if err != nil {
			return err
		}
		raw = updated
		b.Snapshot = snapshot
	}
	row.AttachmentsJSON = raw
	return nil
}

func applyPreviewAttachment(raw string, b FilePreviewBinding, snapshot *eventsv1.FilePreviewSnapshot) (string, error) {
	if err := filepreview.Validate(snapshot); err != nil {
		return "", err
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &rows) != nil || b.Slot < 0 || b.Slot >= len(rows) {
		return "", errors.New("preview slot unavailable")
	}
	row := rows[b.Slot]
	var id, kind string
	if json.Unmarshal(row["file_id"], &id) != nil || json.Unmarshal(row["type"], &kind) != nil || id != b.FileID || kind != b.Kind || snapshot.FileId != id {
		return "", errors.New("preview attachment identity mismatch")
	}
	if _, encrypted := row["e2e_key_wire"]; encrypted {
		return "", errors.New("encrypted preview denied")
	}
	for _, key := range []string{"width", "height", "duration_seconds", "processing_status", "has_thumbnail", "has_converted", "url", "preview_url", "converted_url", "thumb_url", "converted_r2_key", "thumbnail_r2_key"} {
		delete(row, key)
	}
	row["processing_status"], _ = json.Marshal(snapshot.Status)
	if snapshot.Status == "ready" {
		for key, value := range map[string]*int32{"width": snapshot.Width, "height": snapshot.Height, "duration_seconds": snapshot.DurationSeconds} {
			if value != nil {
				row[key], _ = json.Marshal(*value)
			}
		}
		row["has_thumbnail"], _ = json.Marshal(snapshot.HasThumbnail)
		row["has_converted"], _ = json.Marshal(snapshot.HasConverted)
	}
	out, err := json.Marshal(rows)
	return string(out), err
}

type FilePreviewWork struct{ FileID, ChatID uuid.UUID }

func (s *MessagesStore) DeferFilePreview(ctx context.Context, w FilePreviewWork, cancel bool) error {
	if cancel {
		_, err := s.Pool.Exec(ctx, `DELETE FROM messaging_file_preview_pending WHERE file_id=$1 AND chat_id=$2`, w.FileID, w.ChatID)
		return err
	}
	// Purged/deleted owner rows cannot need a pending mutation. Facts themselves
	// remain retained so a delayed send cannot resurrect the file.
	needle, _ := json.Marshal([]map[string]string{{"file_id": w.FileID.String()}})
	if _, err := s.Pool.Exec(ctx, `DELETE FROM messaging_file_preview_pending WHERE file_id=$1 AND chat_id=$2 AND NOT EXISTS(SELECT 1 FROM messages WHERE chat_id=$2 AND deleted_at IS NULL AND NOT is_e2e AND file_preview_bindings @> $3::jsonb)`, w.FileID, w.ChatID, string(needle)); err != nil {
		return err
	}
	_, err := s.Pool.Exec(ctx, `UPDATE messaging_file_preview_pending SET updated_at=clock_timestamp() WHERE file_id=$1 AND chat_id=$2`, w.FileID, w.ChatID)
	return err
}
func (s *MessagesStore) FilePreviewPending(ctx context.Context) ([]FilePreviewWork, error) {
	rows, err := s.Pool.Query(ctx, `SELECT file_id,chat_id FROM messaging_file_preview_pending ORDER BY updated_at,file_id,chat_id LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []FilePreviewWork
	for rows.Next() {
		var w FilePreviewWork
		if err = rows.Scan(&w.FileID, &w.ChatID); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}

func (s *MessagesStore) RefreshFilePreview(ctx context.Context, w FilePreviewWork, space *uuid.UUID) (int, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = lockMessageMutation(ctx, tx, w.ChatID, space, nil); err != nil {
		return 0, err
	}
	// Match insert's message-fence -> snapshot ordering. Lock all candidate
	// messages first; other snapshot rows are never taken during this refresh.
	needle, _ := json.Marshal([]map[string]string{{"file_id": w.FileID.String()}})
	rows, err := tx.Query(ctx, `SELECT id,attachments::text,file_preview_bindings::text,ghost_only FROM messages WHERE chat_id=$1 AND deleted_at IS NULL AND NOT is_e2e AND file_preview_bindings @> $2::jsonb AND EXISTS(SELECT 1 FROM jsonb_array_elements(file_preview_bindings) b WHERE b->>'file_id'=$3 AND (b->'snapshot'->>'metadata_revision')::bigint < (SELECT metadata_revision FROM messaging_file_preview_snapshots WHERE file_id=$3::uuid)) ORDER BY id LIMIT 100 FOR UPDATE`, w.ChatID, string(needle), w.FileID.String())
	if err != nil {
		return 0, err
	}
	type candidate struct {
		id            uuid.UUID
		raw, bindings string
		ghost         bool
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.raw, &c.bindings, &c.ghost); err != nil {
			rows.Close()
			return 0, err
		}
		candidates = append(candidates, c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	// Re-read after message locks: a newer fact must never be overwritten.
	var raw []byte
	if err = tx.QueryRow(ctx, `SELECT payload FROM messaging_file_preview_snapshots WHERE file_id=$1 FOR UPDATE`, w.FileID).Scan(&raw); err != nil {
		return 0, err
	}
	var snapshot eventsv1.FilePreviewSnapshot
	if err = proto.Unmarshal(raw, &snapshot); err != nil {
		return 0, err
	}
	for _, c := range candidates {
		if err = lockMessageMutation(ctx, tx, w.ChatID, space, &c.id); err != nil {
			return 0, err
		}
		var bindings []FilePreviewBinding
		if err = json.Unmarshal([]byte(c.bindings), &bindings); err != nil {
			return 0, err
		}
		updated := c.raw
		changed := false
		for i := range bindings {
			b := &bindings[i]
			if b.FileID != w.FileID.String() || b.Snapshot.GetMetadataRevision() >= snapshot.MetadataRevision {
				continue
			}
			if err = validatePreviewBinding(MessageRow{ID: c.id}, *b); err != nil {
				return 0, err
			}
			if !previewBindingScope(*b, space) {
				return 0, errors.New("file preview destination scope mismatch")
			}
			updated, err = applyPreviewAttachment(updated, *b, &snapshot)
			if err != nil {
				return 0, err
			}
			b.Snapshot = proto.Clone(&snapshot).(*eventsv1.FilePreviewSnapshot)
			changed = true
		}
		if !changed {
			continue
		}
		encoded, _ := json.Marshal(bindings)
		if _, err = tx.Exec(ctx, `UPDATE messages SET attachments=$2::jsonb,file_preview_bindings=$3::jsonb WHERE id=$1 AND deleted_at IS NULL AND NOT is_e2e`, c.id, updated, string(encoded)); err != nil {
			return 0, err
		}
		if !c.ghost {
			event, e := messageevents.NewMessageEditedOutbox(c.id.String(), w.ChatID.String(), false)
			if e != nil {
				return 0, e
			}
			if err = enqueueMessageEvent(ctx, tx, event); err != nil {
				return 0, err
			}
		}
	}
	// A locked snapshot prevents a new event from losing its destination work.
	if len(candidates) < 100 {
		if _, err = tx.Exec(ctx, `DELETE FROM messaging_file_preview_pending WHERE file_id=$1 AND chat_id=$2`, w.FileID, w.ChatID); err != nil {
			return 0, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(candidates), nil
}
