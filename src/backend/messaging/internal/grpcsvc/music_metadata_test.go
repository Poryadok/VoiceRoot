package grpcsvc

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	filev1 "voice.app/voice/file/v1"
)

func TestMusicCanonicalMetadataReplacesCallerTags(t *testing.T) {
	title, artist, album := "File title", "File artist", "File album"
	duration := int32(11)
	meta := &filev1.FileMetadata{Id: "f", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "ready", ScanResult: "clean", SizeBytes: 12, MusicTitle: &title, MusicArtist: &artist, MusicAlbum: &album, DurationSeconds: &duration}
	got, err := canonicalMusicAttachment(`[ {"file_id":"f","type":"music","title":"forged","artist":"forged","album":"forged","duration_seconds":999,"name":"source.mp3"} ]`, meta, false)
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(got), &rows))
	require.Equal(t, title, rows[0]["title"])
	require.Equal(t, artist, rows[0]["artist"])
	require.Equal(t, album, rows[0]["album"])
	require.Equal(t, float64(11), rows[0]["duration_seconds"])
	require.Equal(t, "source.mp3", rows[0]["name"])
}

func TestMusicCanonicalMetadataRejectsStateAndEncryptionMismatch(t *testing.T) {
	for _, meta := range []*filev1.FileMetadata{
		nil,
		{Id: "f", FileType: "audio", Status: "ready", ScanResult: "clean", SizeBytes: 12}, // no Music admission
		{Id: "other", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "ready", ScanResult: "clean"},
		{Id: "f", FileType: "video", Status: "ready", ScanResult: "clean"},
		{Id: "f", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "failed", ScanResult: "clean"},
		{Id: "f", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "ready", ScanResult: "infected"},
		{Id: "f", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "ready", ScanResult: "clean", SizeBytes: 10485761},
		{Id: "f", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "ready", ScanResult: "skipped", IsE2E: true},
	} {
		_, err := canonicalMusicAttachment(`[{"file_id":"f","type":"music"}]`, meta, false)
		require.Error(t, err)
	}
}

func TestEncryptedMusicMetadataStaysOpaque(t *testing.T) {
	meta := &filev1.FileMetadata{Id: "f", Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, FileType: "audio", Status: "ready", ScanResult: "skipped", IsE2E: true, SizeBytes: 42}
	raw := `[{"file_id":"f","type":"music","e2e_key_wire":"opaque-key"}]`
	got, err := canonicalMusicAttachment(raw, meta, true)
	require.NoError(t, err)
	require.JSONEq(t, raw, got)
	for _, field := range []string{"title", "artist", "album", "duration_seconds"} {
		_, err = canonicalMusicAttachment(`[{"file_id":"f","type":"music","`+field+`":"private"}]`, meta, true)
		require.Error(t, err, "plaintext metadata field %s", field)
	}
}
