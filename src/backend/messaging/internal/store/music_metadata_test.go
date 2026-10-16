package store

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestMusicRefreshReplacesMissingTagsAndFencesRevision(t *testing.T) {
	file, chat := uuid.New(), uuid.New()
	binding := MusicAttachmentBinding{FileID: file, SourceChatID: chat, Revision: 1}
	event := &eventsv1.FileProcessed{FileId: file.String(), Status: "ready", Music: &eventsv1.FileMusicMetadata{ChatId: chat.String(), MetadataRevision: 2, SizeBytes: 42, ScanResult: "skipped", DurationSeconds: 7, Title: "new"}}
	raw := `[{"file_id":"` + file.String() + `","type":"music","title":"old","artist":"removed","album":"removed","duration_seconds":5}]`
	got, changed, err := refreshMusicAttachment(raw, binding, event)
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `[{"file_id":"`+file.String()+`","type":"music","title":"new","duration_seconds":7}]`, got)
	binding.Revision = 2
	got, changed, err = refreshMusicAttachment(raw, binding, event)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, raw, got)
}

func TestMusicTombstoneRemovesTagsAndCannotBeResurrected(t *testing.T) {
	file, chat := uuid.New(), uuid.New()
	binding := MusicAttachmentBinding{FileID: file, SourceChatID: chat, Revision: 1}
	raw := `[{"file_id":"` + file.String() + `","type":"music","title":"private old title","artist":"old artist","album":"old album","duration_seconds":7}]`
	tombstone := &eventsv1.FileProcessed{FileId: file.String(), Status: "deleted", Music: &eventsv1.FileMusicMetadata{ChatId: chat.String(), MetadataRevision: 3}}
	got, changed, err := refreshMusicAttachment(raw, binding, tombstone)
	require.NoError(t, err)
	require.True(t, changed)
	require.JSONEq(t, `[{"file_id":"`+file.String()+`","type":"music"}]`, got)
	binding.Revision = 3
	duplicate, changed, err := refreshMusicAttachment(got, binding, tombstone)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, got, duplicate)
	ready := &eventsv1.FileProcessed{FileId: file.String(), Status: "ready", Music: &eventsv1.FileMusicMetadata{ChatId: chat.String(), MetadataRevision: 2, Title: "stale", SizeBytes: 42, ScanResult: "clean", DurationSeconds: 7}}
	duplicate, changed, err = refreshMusicAttachment(got, binding, ready)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, got, duplicate)
}

func TestMusicRefreshRejectsDifferentFileChatAndUnsafeSnapshot(t *testing.T) {
	file, chat := uuid.New(), uuid.New()
	for _, mutate := range []func(*eventsv1.FileProcessed){
		func(e *eventsv1.FileProcessed) { e.FileId = uuid.NewString() },
		func(e *eventsv1.FileProcessed) { e.Music.ChatId = uuid.NewString() },
		func(e *eventsv1.FileProcessed) { e.Status = "deleted" },
		func(e *eventsv1.FileProcessed) { e.Music.ScanResult = "infected" },
		func(e *eventsv1.FileProcessed) { e.Music.ScanResult = "pending" },
		func(e *eventsv1.FileProcessed) { e.Music.ScanResult = "error" },
		func(e *eventsv1.FileProcessed) { e.Music.SizeBytes = 10485761 },
		func(e *eventsv1.FileProcessed) { e.Music.Title = "unsafe\nlabel" },
	} {
		event := &eventsv1.FileProcessed{FileId: file.String(), Status: "ready", Music: &eventsv1.FileMusicMetadata{ChatId: chat.String(), MetadataRevision: 2, SizeBytes: 42, ScanResult: "clean", DurationSeconds: 7}}
		mutate(event)
		_, changed, err := refreshMusicAttachment(`[{"file_id":"`+file.String()+`","type":"music"}]`, MusicAttachmentBinding{FileID: file, SourceChatID: chat, Revision: 1}, event)
		require.Error(t, err)
		require.False(t, changed)
	}
}
