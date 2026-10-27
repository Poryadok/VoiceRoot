package grpcsvc

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/audioproc"
	"voice/backend/file/internal/store"
)

const audioIntent = filev1.UploadIntent(7)

func startAudioTestPostgres(t *testing.T, ctx context.Context) *store.FilesStore {
	t.Helper()
	pool := startFileGatePostgres(t, ctx)
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000014_audio_intent.up.sql"))
	return store.NewFilesStore(pool)
}

func TestAudioUploadAdmission(t *testing.T) {
	ctx := context.Background()
	files := startAudioTestPostgres(t, ctx)
	profile, chat := uuid.New(), uuid.New()
	client := dialEventTestGRPC(t, New(Deps{Files: files, Presigner: eventTestPresigner{}, ChatGuard: e2eGateGuard{members: map[uuid.UUID][]uuid.UUID{chat: {profile}}, e2eEnabled: map[uuid.UUID]bool{chat: false}, chatTypes: map[uuid.UUID]string{chat: "dm"}}}))
	authed := fileGateCtx(ctx, uuid.New(), profile)
	for _, mime := range []string{"audio/mpeg", "audio/flac", "audio/ogg", "audio/wav"} {
		_, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{Intent: audioIntent, OriginalName: "clip", MimeType: mime, SizeBytes: audioproc.MaxBytes, ContextChat: chatDMRefGate(chat)})
		require.NoError(t, err, mime)
	}
	for _, tc := range []struct {
		mime string
		size int64
		chat bool
	}{
		{"audio/mp4", 1, true}, {"video/mp4", 1, true}, {"audio/mpeg", audioproc.MaxBytes + 1, true}, {"audio/mpeg", 1, false},
	} {
		req := &filev1.RequestUploadRequest{Intent: audioIntent, OriginalName: "clip", MimeType: tc.mime, SizeBytes: tc.size}
		if tc.chat {
			req.ContextChat = chatDMRefGate(chat)
		}
		_, err := client.RequestUpload(authed, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err), "%+v", tc)
	}
}

func TestConfirmAudioPersistsMetadataWithoutMusicEvent(t *testing.T) {
	ctx := context.Background()
	files := startAudioTestPostgres(t, ctx)
	pool := files.Pool
	profile, chat := uuid.New(), uuid.New()
	extractor := &musicExtractorSpy{}
	reader := eventObjectReader{}
	client := dialEventTestGRPC(t, New(Deps{Files: files, Presigner: eventTestPresigner{}, Reader: reader, Scanner: eventScanner("clean"), AudioProcessor: extractor, ChatGuard: e2eGateGuard{members: map[uuid.UUID][]uuid.UUID{chat: {profile}}, e2eEnabled: map[uuid.UUID]bool{chat: false}, chatTypes: map[uuid.UUID]string{chat: "dm"}}}))
	authed := fileGateCtx(ctx, uuid.New(), profile)
	payload := []byte("validated audio fixture")
	upload, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{Intent: audioIntent, OriginalName: "clip.ogg", MimeType: "audio/ogg", SizeBytes: int64(len(payload)), ContextChat: chatDMRefGate(chat)})
	require.NoError(t, err)
	reader[upload.GetUploadResponse().GetR2Key()] = payload
	id := upload.GetUploadResponse().GetFileId()
	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: id, Sha256Hash: shaHex(payload)})
	require.NoError(t, err)
	meta := confirmed.GetFileMetadata()
	require.Equal(t, audioIntent, meta.GetIntent())
	require.Equal(t, "ready", meta.GetStatus())
	require.Equal(t, "Extracted title", meta.GetMusicTitle())
	require.EqualValues(t, 12, meta.GetDurationSeconds())
	require.Zero(t, meta.GetMusicMetadataRevision())
	require.Equal(t, 1, extractor.calls)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_music_event_outbox WHERE file_id=$1`, id).Scan(&count))
	require.Zero(t, count)
	_, err = client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: id, Sha256Hash: shaHex(payload)})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestEncryptedAudioSkipsExtraction(t *testing.T) {
	ctx := context.Background()
	files := startAudioTestPostgres(t, ctx)
	profile, chat := uuid.New(), uuid.New()
	extractor := &musicExtractorSpy{}
	reader := eventObjectReader{}
	client := dialEventTestGRPC(t, New(Deps{Files: files, Presigner: eventTestPresigner{}, Reader: reader, AudioProcessor: extractor, ChatGuard: e2eGateGuard{members: map[uuid.UUID][]uuid.UUID{chat: {profile}}, e2eEnabled: map[uuid.UUID]bool{chat: true}, chatTypes: map[uuid.UUID]string{chat: "dm"}}}))
	authed := fileGateCtx(ctx, uuid.New(), profile)
	encrypted := true
	payload := []byte("opaque ciphertext")
	upload, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{Intent: audioIntent, OriginalName: "clip.ogg", MimeType: "audio/ogg", SizeBytes: int64(len(payload)), ContextChat: chatDMRefGate(chat), IsE2E: &encrypted})
	require.NoError(t, err)
	reader[upload.GetUploadResponse().GetR2Key()] = payload
	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: upload.GetUploadResponse().GetFileId(), Sha256Hash: shaHex(payload)})
	require.NoError(t, err)
	require.Equal(t, audioIntent, confirmed.GetFileMetadata().GetIntent())
	require.Equal(t, "ready", confirmed.GetFileMetadata().GetStatus())
	require.Zero(t, extractor.calls)
	require.Empty(t, confirmed.GetFileMetadata().GetMusicTitle())
	require.Nil(t, confirmed.GetFileMetadata().DurationSeconds)
}
