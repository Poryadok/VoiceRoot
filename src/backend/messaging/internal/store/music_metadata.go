package store

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"math"
	"unicode"
	"unicode/utf8"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/messaging/internal/messageevents"
)

// MusicAttachmentBinding is private Messaging state established from the
// authenticated File metadata lookup, never from caller attachment JSON.
// SourceChatID is not exposed on forwarded messages in another chat.
type MusicAttachmentBinding struct {
	FileID       uuid.UUID `json:"file_id"`
	SourceChatID uuid.UUID `json:"source_chat_id"`
	Revision     uint64    `json:"revision"`
	Status       string    `json:"status"`
	SizeBytes    int64     `json:"size_bytes"`
	ScanResult   string    `json:"scan_result"`
	Duration     int64     `json:"duration_seconds"`
	Title        string    `json:"title"`
	Artist       string    `json:"artist"`
	Album        string    `json:"album"`
}

type musicFileSnapshot struct {
	FileID       uuid.UUID
	SourceChatID uuid.UUID
	Revision     int64
	Status       string
	SizeBytes    int64
	ScanResult   string
	Duration     int64
	Title        string
	Artist       string
	Album        string
}

func musicSnapshotFromEvent(e *eventsv1.FileProcessed) (musicFileSnapshot, error) {
	if err := ValidateMusicSnapshot(e); err != nil {
		return musicFileSnapshot{}, err
	}
	file, _ := uuid.Parse(e.GetFileId())
	chat, _ := uuid.Parse(e.GetMusic().GetChatId())
	m := e.GetMusic()
	return musicFileSnapshot{
		FileID: file, SourceChatID: chat, Revision: int64(m.GetMetadataRevision()), Status: e.GetStatus(),
		SizeBytes: int64(m.GetSizeBytes()), ScanResult: m.GetScanResult(), Duration: int64(m.GetDurationSeconds()),
		Title: m.GetTitle(), Artist: m.GetArtist(), Album: m.GetAlbum(),
	}, nil
}

func (s musicFileSnapshot) event() *eventsv1.FileProcessed {
	return &eventsv1.FileProcessed{
		FileId: s.FileID.String(), Status: s.Status,
		Music: &eventsv1.FileMusicMetadata{
			ChatId: s.SourceChatID.String(), MetadataRevision: uint64(s.Revision), SizeBytes: s.SizeBytes,
			ScanResult: s.ScanResult, DurationSeconds: int32(s.Duration), Title: s.Title, Artist: s.Artist, Album: s.Album,
		},
	}
}

func (s musicFileSnapshot) equal(other musicFileSnapshot) bool {
	return s.FileID == other.FileID && s.SourceChatID == other.SourceChatID && s.Revision == other.Revision &&
		s.Status == other.Status && s.SizeBytes == other.SizeBytes && s.ScanResult == other.ScanResult &&
		s.Duration == other.Duration && s.Title == other.Title && s.Artist == other.Artist && s.Album == other.Album
}

func musicSnapshotFromBinding(b *MusicAttachmentBinding) (musicFileSnapshot, error) {
	if b == nil || b.Revision == 0 || b.Revision > math.MaxInt64 || b.FileID == uuid.Nil || b.SourceChatID == uuid.Nil || b.SizeBytes < 0 || b.Duration < 0 || b.Duration > math.MaxInt32 {
		return musicFileSnapshot{}, errors.New("invalid music admission snapshot")
	}
	return musicSnapshotFromEvent(&eventsv1.FileProcessed{
		FileId: b.FileID.String(), Status: b.Status,
		Music: &eventsv1.FileMusicMetadata{
			ChatId: b.SourceChatID.String(), MetadataRevision: b.Revision, SizeBytes: b.SizeBytes,
			ScanResult: b.ScanResult, DurationSeconds: int32(b.Duration), Title: b.Title, Artist: b.Artist, Album: b.Album,
		},
	})
}

// PersistMusicSnapshot keeps the latest authoritative File snapshot even when
// the event arrives before the first message or forwarded copy is inserted.
// The returned value is the current revision, so a delayed event still drives
// convergence from a newer snapshot already stored here.
func (s *MessagesStore) PersistMusicSnapshot(ctx context.Context, e *eventsv1.FileProcessed) (*eventsv1.FileProcessed, error) {
	if s == nil || s.Pool == nil {
		return nil, errors.New("messages store: pool not configured")
	}
	incoming, err := musicSnapshotFromEvent(e)
	if err != nil {
		return nil, err
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(context.Background())
	current, err := persistMusicSnapshotTx(ctx, tx, incoming)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return current.event(), nil
}

func persistMusicSnapshotTx(ctx context.Context, tx pgx.Tx, incoming musicFileSnapshot) (musicFileSnapshot, error) {
	_, err := tx.Exec(ctx, `INSERT INTO messaging_music_file_snapshots
 (file_id,source_chat_id,metadata_revision,status,size_bytes,scan_result,duration_seconds,title,artist,album)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
 ON CONFLICT(file_id) DO UPDATE SET source_chat_id=EXCLUDED.source_chat_id,
 metadata_revision=EXCLUDED.metadata_revision,status=EXCLUDED.status,size_bytes=EXCLUDED.size_bytes,
 scan_result=EXCLUDED.scan_result,duration_seconds=EXCLUDED.duration_seconds,title=EXCLUDED.title,
 artist=EXCLUDED.artist,album=EXCLUDED.album,updated_at=clock_timestamp()
 WHERE messaging_music_file_snapshots.source_chat_id=EXCLUDED.source_chat_id
 AND messaging_music_file_snapshots.metadata_revision<EXCLUDED.metadata_revision`,
		incoming.FileID, incoming.SourceChatID, incoming.Revision, incoming.Status, incoming.SizeBytes,
		incoming.ScanResult, incoming.Duration, incoming.Title, incoming.Artist, incoming.Album)
	if err != nil {
		return musicFileSnapshot{}, err
	}
	var current musicFileSnapshot
	err = tx.QueryRow(ctx, `SELECT file_id,source_chat_id,metadata_revision,status,size_bytes,scan_result,duration_seconds,title,artist,album
 FROM messaging_music_file_snapshots WHERE file_id=$1 FOR UPDATE`, incoming.FileID).Scan(
		&current.FileID, &current.SourceChatID, &current.Revision, &current.Status, &current.SizeBytes,
		&current.ScanResult, &current.Duration, &current.Title, &current.Artist, &current.Album)
	if err != nil {
		return musicFileSnapshot{}, err
	}
	if current.SourceChatID != incoming.SourceChatID || current.Revision < incoming.Revision ||
		(current.Revision == incoming.Revision && !current.equal(incoming)) {
		return musicFileSnapshot{}, errors.New("conflicting music file snapshot")
	}
	return current, nil
}

func (s *MessagesStore) reconcileMusicInsert(ctx context.Context, tx pgx.Tx, row *MessageRow) error {
	if row.MusicBinding == nil || row.IsE2E || row.ContentType != "music" {
		return nil
	}
	binding := *row.MusicBinding
	incoming, err := musicSnapshotFromBinding(&binding)
	if err != nil {
		return err
	}
	current, err := persistMusicSnapshotTx(ctx, tx, incoming)
	if err != nil {
		return err
	}
	if current.Revision > int64(binding.Revision) {
		updated, changed, err := refreshMusicAttachment(row.AttachmentsJSON, binding, current.event())
		if err != nil {
			return err
		}
		if !changed {
			return errors.New("latest music snapshot did not advance admission")
		}
		row.AttachmentsJSON = updated
		binding.Revision = uint64(current.Revision)
	}
	row.MusicBinding = &binding
	return nil
}

// ValidateMusicSnapshot shares the same closed status/size/tag validation as
// the transactional mutation, before the consumer enumerates any destinations.
func ValidateMusicSnapshot(e *eventsv1.FileProcessed) error {
	file, err := uuid.Parse(e.GetFileId())
	if err != nil {
		return err
	}
	chat, err := uuid.Parse(e.GetMusic().GetChatId())
	if err != nil {
		return err
	}
	_, _, err = refreshMusicAttachment(`[{"file_id":"`+file.String()+`","type":"music"}]`, MusicAttachmentBinding{FileID: file, SourceChatID: chat}, e)
	return err
}

// MusicRefreshChats returns only destinations already bound at authenticated
// send admission. A forwarded message can belong to a different chat without
// trusting a chat ID from public attachment JSON.
func (s *MessagesStore) MusicRefreshChats(ctx context.Context, e *eventsv1.FileProcessed) ([]uuid.UUID, error) {
	file, err := uuid.Parse(e.GetFileId())
	if err != nil {
		return nil, err
	}
	source, err := uuid.Parse(e.GetMusic().GetChatId())
	if err != nil {
		return nil, err
	}
	rows, err := s.Pool.Query(ctx, `SELECT DISTINCT chat_id FROM messages WHERE music_file_id=$1 AND music_source_chat_id=$2 AND deleted_at IS NULL AND NOT is_e2e ORDER BY chat_id`, file, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var chats []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		chats = append(chats, id)
	}
	return chats, rows.Err()
}

// RefreshMusicMetadata commits a bounded batch and its message_update outbox
// atomically. The caller resolves the destination's authoritative Space scope
// and repeats only while a full batch was changed. No end-user is fabricated.
func (s *MessagesStore) RefreshMusicMetadata(ctx context.Context, chat uuid.UUID, space *uuid.UUID, e *eventsv1.FileProcessed) (int, error) {
	file, err := uuid.Parse(e.GetFileId())
	if err != nil {
		return 0, err
	}
	source, err := uuid.Parse(e.GetMusic().GetChatId())
	if err != nil {
		return 0, err
	}
	revision := e.GetMusic().GetMetadataRevision()
	if revision == 0 || revision > math.MaxInt64 {
		return 0, errors.New("invalid music revision")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	if err = lockMessageMutation(ctx, tx, chat, space, nil); err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT id,attachments::text,music_metadata_revision,ghost_only FROM messages
 WHERE chat_id=$1 AND music_file_id=$2 AND music_source_chat_id=$3
 AND music_metadata_revision<$4 AND deleted_at IS NULL AND NOT is_e2e AND content_type='music'
 ORDER BY id FOR UPDATE LIMIT 100`, chat, file, source, int64(revision))
	if err != nil {
		return 0, err
	}
	type candidate struct {
		id       uuid.UUID
		raw      string
		revision uint64
		ghost    bool
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err = rows.Scan(&c.id, &c.raw, &c.revision, &c.ghost); err != nil {
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
	for _, c := range candidates {
		if err = lockMessageMutation(ctx, tx, chat, space, &c.id); err != nil {
			return 0, err
		}
		updated, changed, refreshErr := refreshMusicAttachment(c.raw, MusicAttachmentBinding{FileID: file, SourceChatID: source, Revision: c.revision}, e)
		if refreshErr != nil {
			return 0, refreshErr
		}
		if !changed {
			return 0, errors.New("music revision did not advance")
		}
		tag, updateErr := tx.Exec(ctx, `UPDATE messages SET attachments=$2::jsonb,music_metadata_revision=$3 WHERE id=$1 AND deleted_at IS NULL AND NOT is_e2e`, c.id, updated, int64(revision))
		if updateErr != nil {
			return 0, updateErr
		}
		if tag.RowsAffected() != 1 {
			return 0, pgx.ErrNoRows
		}
		if !c.ghost {
			event, eventErr := messageevents.NewMessageEditedOutbox(c.id.String(), chat.String(), false)
			if eventErr != nil {
				return 0, eventErr
			}
			if err = enqueueMessageEvent(ctx, tx, event); err != nil {
				return 0, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(candidates), nil
}

func refreshMusicAttachment(raw string, b MusicAttachmentBinding, e *eventsv1.FileProcessed) (string, bool, error) {
	invalid := errors.New("invalid music metadata refresh")
	m := e.GetMusic()
	if b.FileID == uuid.Nil || b.SourceChatID == uuid.Nil || e.GetFileId() != b.FileID.String() || m.GetChatId() != b.SourceChatID.String() || m.GetMetadataRevision() == 0 || m.GetMetadataRevision() > math.MaxInt64 {
		return "", false, invalid
	}
	tombstone := e.GetStatus() == "deleted" || e.GetStatus() == "expired"
	if tombstone {
		if m.GetSizeBytes() != 0 || m.GetDurationSeconds() != 0 || m.GetScanResult() != "" || m.GetTitle() != "" || m.GetArtist() != "" || m.GetAlbum() != "" {
			return "", false, invalid
		}
	} else if e.GetStatus() != "ready" || m.GetSizeBytes() <= 0 || m.GetSizeBytes() > 10485760 || m.GetDurationSeconds() <= 0 || (m.GetScanResult() != "clean" && m.GetScanResult() != "skipped") {
		return "", false, invalid
	}
	for _, v := range []string{m.GetTitle(), m.GetArtist(), m.GetAlbum()} {
		if len(v) > 1024 || !utf8.ValidString(v) {
			return "", false, invalid
		}
		for _, r := range v {
			if unicode.IsControl(r) {
				return "", false, invalid
			}
		}
	}
	var attachments []map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &attachments) != nil || len(attachments) != 1 {
		return "", false, invalid
	}
	var id, kind string
	if json.Unmarshal(attachments[0]["file_id"], &id) != nil || json.Unmarshal(attachments[0]["type"], &kind) != nil || id != b.FileID.String() || kind != "music" {
		return "", false, invalid
	}
	if _, encrypted := attachments[0]["e2e_key_wire"]; encrypted {
		return "", false, invalid
	}
	if m.GetMetadataRevision() <= b.Revision {
		return raw, false, nil
	}
	for _, key := range []string{"title", "artist", "album", "duration_seconds"} {
		delete(attachments[0], key)
	}
	for key, value := range map[string]string{"title": m.GetTitle(), "artist": m.GetArtist(), "album": m.GetAlbum()} {
		if value != "" {
			attachments[0][key], _ = json.Marshal(value)
		}
	}
	if !tombstone {
		attachments[0]["duration_seconds"], _ = json.Marshal(m.GetDurationSeconds())
	}
	encoded, err := json.Marshal(attachments)
	return string(encoded), err == nil, err
}
