package fileevents

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	"unicode"
	"unicode/utf8"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/file/internal/audioproc"
)

func NewMusicProcessed(fileID, chatID string, revision uint64, size int64, scan string, m audioproc.Metadata) (*eventsv1.FileStreamEvent, error) {
	invalid := errors.New("invalid music processing authority")
	if id, err := uuid.Parse(fileID); err != nil || id == uuid.Nil {
		return nil, invalid
	}
	if id, err := uuid.Parse(chatID); err != nil || id == uuid.Nil {
		return nil, invalid
	}
	if revision == 0 || size <= 0 || size > audioproc.MaxBytes || (scan != "clean" && scan != "skipped") || m.DurationSeconds <= 0 {
		return nil, invalid
	}
	for _, value := range []string{m.Title, m.Artist, m.Album} {
		if len(value) > 1024 || !utf8.ValidString(value) {
			return nil, invalid
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return nil, invalid
			}
		}
	}
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileProcessed{FileProcessed: &eventsv1.FileProcessed{
		FileId: fileID, Status: "ready", Music: &eventsv1.FileMusicMetadata{ChatId: chatID, MetadataRevision: revision, SizeBytes: size, ScanResult: scan, Title: m.Title, Artist: m.Artist, Album: m.Album, DurationSeconds: m.DurationSeconds},
	}}
	return env, nil
}

// Tombstones carry no extracted tags. Their durable revision prevents a delayed
// ready snapshot from restoring metadata after this event has been consumed.
func NewMusicTombstone(fileID, chatID string, revision uint64, status string) (*eventsv1.FileStreamEvent, error) {
	invalid := errors.New("invalid music tombstone authority")
	if id, err := uuid.Parse(fileID); err != nil || id == uuid.Nil {
		return nil, invalid
	}
	if id, err := uuid.Parse(chatID); err != nil || id == uuid.Nil {
		return nil, invalid
	}
	if revision == 0 || (status != "deleted" && status != "expired") {
		return nil, invalid
	}
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileProcessed{FileProcessed: &eventsv1.FileProcessed{FileId: fileID, Status: status, Music: &eventsv1.FileMusicMetadata{ChatId: chatID, MetadataRevision: revision}}}
	return env, nil
}

// PublishMusicEvent publishes an already committed outbox snapshot. Its event
// ID and revision survive retries; the consumer is transactionally idempotent.
func (p *JetStreamPublisher) PublishMusicEvent(ctx context.Context, payload []byte) error {
	var event eventsv1.FileStreamEvent
	if len(payload) > 8192 || proto.Unmarshal(payload, &event) != nil || event.GetFileProcessed().GetMusic() == nil {
		return errors.New("invalid music outbox payload")
	}
	return p.publishProto(ctx, subjectFileProcessed, &event)
}
