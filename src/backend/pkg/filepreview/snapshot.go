// Package filepreview validates the redacted File-owned preview wire contract.
// Validation confers no reference or viewer authority.
package filepreview

import (
	"errors"
	"math"

	"github.com/google/uuid"
	eventsv1 "voice.app/voice/events/v1"
)

func Terminal(status string) bool { return status == "deleted" || status == "expired" }

func AttachmentKind(kind string) bool {
	switch kind {
	case "image", "photo", "video", "audio", "voice", "voice_message", "document", "file", "other", "gif", "sticker", "video_note":
		return true
	default:
		return false
	}
}

func Validate(s *eventsv1.FilePreviewSnapshot) error {
	invalid := errors.New("invalid file preview snapshot")
	if s == nil || len(s.ProtoReflect().GetUnknown()) != 0 || s.ProtocolVersion != 1 || s.MetadataRevision == 0 || s.MetadataRevision > math.MaxInt64 || s.IsE2E {
		return invalid
	}
	id, err := uuid.Parse(s.FileId)
	if err != nil || id == uuid.Nil || id.String() != s.FileId {
		return invalid
	}
	switch s.Status {
	case "failed", "deleted", "expired", "pending_upload", "processing":
		if s.FileType != "" || s.UploadIntent != "" || s.SizeBytes != 0 || s.ScanResult != "" || s.Width != nil || s.Height != nil || s.DurationSeconds != nil || s.HasThumbnail || s.HasConverted {
			return invalid
		}
	case "ready":
		switch s.FileType {
		case "image", "video", "audio", "document", "other":
		default:
			return invalid
		}
		// Only existing persisted intents are recognized; Music has a separate protocol.
		if s.UploadIntent != "" {
			return invalid
		}
		if s.SizeBytes <= 0 || (s.ScanResult != "clean" && s.ScanResult != "skipped") {
			return invalid
		}
		for _, n := range []*int32{s.Width, s.Height, s.DurationSeconds} {
			if n != nil && *n < 0 {
				return invalid
			}
		}
		if (s.Width != nil || s.Height != nil) && s.FileType != "image" && s.FileType != "video" {
			return invalid
		}
		if s.HasThumbnail && s.FileType != "image" && s.FileType != "video" && s.FileType != "document" {
			return invalid
		}
		if s.DurationSeconds != nil && s.FileType != "audio" && s.FileType != "video" {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}

func ValidateEvent(e *eventsv1.FileProcessed) error {
	if e == nil || e.Music != nil || e.ConvertedR2Key != nil || e.ThumbnailR2Key != nil || len(e.ProtoReflect().GetUnknown()) != 0 {
		return errors.New("invalid preview envelope")
	}
	if err := Validate(e.Preview); err != nil {
		return err
	}
	if e.FileId != e.Preview.FileId || e.Status != e.Preview.Status {
		return errors.New("preview envelope mismatch")
	}
	return nil
}
