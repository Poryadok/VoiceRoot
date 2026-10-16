package grpcsvc

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/messaging/internal/store"
)

// canonicalMusicAttachment is shared by admission and authoritative processing
// refresh. Display tags supplied by a plaintext sender are never authoritative.
func canonicalMusicAttachment(raw string, meta *filev1.FileMetadata, encrypted bool) (string, error) {
	deny := func() (string, error) {
		return "", status.Error(codes.FailedPrecondition, "music metadata unavailable")
	}
	var rows []map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &rows) != nil || len(rows) != 1 || meta == nil {
		return deny()
	}
	var id, kind string
	if json.Unmarshal(rows[0]["file_id"], &id) != nil || json.Unmarshal(rows[0]["type"], &kind) != nil || id != meta.GetId() || kind != "music" {
		return deny()
	}
	if meta.GetIntent() != filev1.UploadIntent_UPLOAD_INTENT_MUSIC || meta.GetFileType() != "audio" || meta.GetStatus() != "ready" || (meta.GetScanResult() != "clean" && meta.GetScanResult() != "skipped") || meta.GetSizeBytes() <= 0 || meta.GetSizeBytes() > 10*1024*1024 || meta.GetIsE2E() != encrypted {
		return deny()
	}
	for _, key := range []string{"title", "artist", "album", "duration_seconds"} {
		if _, ok := rows[0][key]; encrypted && ok {
			return deny()
		}
		delete(rows[0], key)
	}
	if !encrypted {
		for key, value := range map[string]string{"title": meta.GetMusicTitle(), "artist": meta.GetMusicArtist(), "album": meta.GetMusicAlbum()} {
			if value != "" {
				encoded, _ := json.Marshal(value)
				rows[0][key] = encoded
			}
		}
		if meta.GetDurationSeconds() > 0 {
			encoded, _ := json.Marshal(meta.GetDurationSeconds())
			rows[0]["duration_seconds"] = encoded
		}
	}
	encoded, err := json.Marshal(rows)
	return string(encoded), err
}

func (s *MessagingGRPC) enrichMusicAttachment(ctx context.Context, raw, contentType string, encrypted bool) (string, *store.MusicAttachmentBinding, error) {
	if contentType != "music" {
		return raw, nil, nil
	}
	var rows []messageAttachment
	if json.Unmarshal([]byte(raw), &rows) != nil || len(rows) != 1 || s.Files == nil {
		return "", nil, status.Error(codes.FailedPrecondition, "music metadata unavailable")
	}
	response, err := s.Files.GetBulkMetadata(ctx, &filev1.GetBulkMetadataRequest{FileIds: []string{rows[0].FileID}})
	if err != nil {
		return "", nil, status.Error(codes.Unavailable, "music metadata unavailable")
	}
	meta := response.GetBulkFileMetadata().GetByFileId()[rows[0].FileID]
	canonical, err := canonicalMusicAttachment(raw, meta, encrypted)
	if err != nil || encrypted {
		return canonical, nil, err
	}
	fileID, fileErr := uuid.Parse(meta.GetId())
	chatID, chatErr := uuid.Parse(meta.GetChat().GetId())
	if fileErr != nil || chatErr != nil || fileID == uuid.Nil || chatID == uuid.Nil || meta.GetMusicMetadataRevision() == 0 || meta.GetMusicMetadataRevision() > uint64(1<<63-1) || meta.GetDurationSeconds() <= 0 {
		return "", nil, status.Error(codes.FailedPrecondition, "music metadata unavailable")
	}
	return canonical, &store.MusicAttachmentBinding{
		FileID: fileID, SourceChatID: chatID, Revision: meta.GetMusicMetadataRevision(),
		Status: meta.GetStatus(), SizeBytes: int64(meta.GetSizeBytes()), ScanResult: meta.GetScanResult(),
		Duration: int64(meta.GetDurationSeconds()), Title: meta.GetMusicTitle(), Artist: meta.GetMusicArtist(), Album: meta.GetMusicAlbum(),
	}, nil
}
