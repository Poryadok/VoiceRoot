package fileevents

import (
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"voice/backend/file/internal/audioproc"
)

func TestMusicProcessedCarriesPersistedAuthority(t *testing.T) {
	file, chat := uuid.NewString(), uuid.NewString()
	event, err := NewMusicProcessed(file, chat, 2, 123, "clean", audioproc.Metadata{Title: "Title", Artist: "Artist", Album: "Album", DurationSeconds: 12})
	require.NoError(t, err)
	processed := event.GetFileProcessed()
	require.Equal(t, file, processed.GetFileId())
	require.Equal(t, "ready", processed.GetStatus())
	music := processed.GetMusic()
	require.Equal(t, chat, music.GetChatId())
	require.EqualValues(t, 2, music.GetMetadataRevision())
	require.Equal(t, "Title", music.GetTitle())
	require.EqualValues(t, 12, music.GetDurationSeconds())
}

func TestMusicProcessedRejectsUnsafeAuthority(t *testing.T) {
	for _, scan := range []string{"pending", "infected", "error"} {
		_, err := NewMusicProcessed(uuid.NewString(), uuid.NewString(), 1, 12, scan, audioproc.Metadata{DurationSeconds: 1})
		require.Error(t, err)
	}
	_, err := NewMusicProcessed(uuid.NewString(), uuid.NewString(), 0, 12, "clean", audioproc.Metadata{DurationSeconds: 1})
	require.Error(t, err)
	_, err = NewMusicProcessed(uuid.NewString(), uuid.NewString(), 1, 10485761, "clean", audioproc.Metadata{DurationSeconds: 1})
	require.Error(t, err)
}

func TestMusicTombstoneContainsOnlyRevocationAuthority(t *testing.T) {
	for _, status := range []string{"deleted", "expired"} {
		event, err := NewMusicTombstone(uuid.NewString(), uuid.NewString(), 7, status)
		require.NoError(t, err)
		m := event.GetFileProcessed().GetMusic()
		require.EqualValues(t, 7, m.GetMetadataRevision())
		require.Empty(t, m.GetTitle())
		require.Empty(t, m.GetArtist())
		require.Empty(t, m.GetAlbum())
		require.Zero(t, m.GetDurationSeconds())
		require.Zero(t, m.GetSizeBytes())
		require.Empty(t, m.GetScanResult())
	}
	for _, ids := range [][2]string{{uuid.Nil.String(), uuid.NewString()}, {uuid.NewString(), uuid.Nil.String()}} {
		_, err := NewMusicTombstone(ids[0], ids[1], 1, "deleted")
		require.Error(t, err)
		_, err = NewMusicProcessed(ids[0], ids[1], 1, 12, "skipped", audioproc.Metadata{DurationSeconds: 1})
		require.Error(t, err)
	}
	_, err := NewMusicTombstone(uuid.NewString(), uuid.NewString(), 1, "ready")
	require.Error(t, err)
}
