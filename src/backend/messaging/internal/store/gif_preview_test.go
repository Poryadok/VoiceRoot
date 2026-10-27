package store

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"testing"
	eventsv1 "voice.app/voice/events/v1"
)

func TestGIFPreviewV2ConsumerKeepsLegacyV1AndTerminalFence(t *testing.T) {
	ctx := context.Background()
	pool := startPostgresForStoreTest(t, ctx)
	defer pool.Close()
	seedMessagingSchema(t, ctx, pool)
	s := &MessagesStore{Pool: pool}
	gif, legacy := uuid.New(), uuid.New()
	snapshot := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 2, FileId: gif.String(), MetadataRevision: 1, Status: "ready", FileType: "video", UploadIntent: "gif", SizeBytes: 42, ScanResult: "skipped", HasConverted: true, HasThumbnail: true}
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(snapshot)))
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(previewTestSnapshot(legacy, 1, "ready"))))
	terminal := &eventsv1.FilePreviewSnapshot{ProtocolVersion: 2, FileId: gif.String(), MetadataRevision: 2, Status: "deleted"}
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(terminal)))
	require.NoError(t, s.PersistFilePreview(ctx, previewTestEvent(snapshot)), "stale replay must not resurrect")
	fresh := proto.Clone(snapshot).(*eventsv1.FilePreviewSnapshot)
	fresh.MetadataRevision = 3
	require.Error(t, s.PersistFilePreview(ctx, previewTestEvent(fresh)), "terminal fence survives new protocol")
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM messaging_file_preview_snapshots`).Scan(&count))
	require.Equal(t, 2, count)
	var raw []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM messaging_file_preview_snapshots WHERE file_id=$1`, gif).Scan(&raw))
	var saved eventsv1.FilePreviewSnapshot
	require.NoError(t, proto.Unmarshal(raw, &saved))
	require.EqualValues(t, 2, saved.ProtocolVersion)
	require.Equal(t, "deleted", saved.Status)
	require.Empty(t, saved.UploadIntent)
}
