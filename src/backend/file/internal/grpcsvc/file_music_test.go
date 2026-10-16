package grpcsvc

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/audioproc"
	"voice/backend/file/internal/store"
)

type musicExtractorSpy struct{ calls int }

func (s *musicExtractorSpy) Extract(context.Context, []byte) (audioproc.Metadata, error) {
	s.calls++
	return audioproc.Metadata{Title: "Extracted title", Artist: "Extracted artist", Album: "Extracted album", DurationSeconds: 12}, nil
}

func TestMusicUploadEnforcesStoredLimitBeforeTicket(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	client := dialEventTestGRPC(t, New(Deps{Files: store.NewFilesStore(pool), Presigner: eventTestPresigner{}}))
	_, err := client.RequestUpload(fileGateCtx(ctx, uuid.New(), uuid.New()), &filev1.RequestUploadRequest{Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, OriginalName: "large.mp3", MimeType: "audio/mpeg", SizeBytes: audioproc.MaxBytes + 1, ContextChat: chatDMRefGate(uuid.New())})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestConfirmMusicPersistsAuthoritativeMetadataAndPublishesProcessed(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	events := &spyFileEvents{}
	extractor := &musicExtractorSpy{}
	reader := eventObjectReader{}
	svc := New(Deps{Files: store.NewFilesStore(pool), Presigner: eventTestPresigner{}, Reader: reader, Scanner: eventScanner("clean"), AudioProcessor: extractor, Events: events})
	client := dialEventTestGRPC(t, svc)
	profile, chat := uuid.New(), uuid.New()
	svc.chatGuard = e2eGateGuard{members: map[uuid.UUID][]uuid.UUID{chat: {profile}}, e2eEnabled: map[uuid.UUID]bool{chat: false}, chatTypes: map[uuid.UUID]string{chat: "dm"}}
	authed := fileGateCtx(ctx, uuid.New(), profile)
	payload := []byte("validated audio fixture")
	upload, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, OriginalName: "music.mp3", MimeType: "audio/mpeg", SizeBytes: int64(len(payload)), ContextChat: chatDMRefGate(chat)})
	require.NoError(t, err)
	reader[upload.GetUploadResponse().GetR2Key()] = payload
	id := upload.GetUploadResponse().GetFileId()
	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: id, Sha256Hash: shaHex(payload)})
	require.NoError(t, err)
	m := confirmed.GetFileMetadata()
	require.Equal(t, "ready", m.GetStatus())
	require.Equal(t, filev1.UploadIntent_UPLOAD_INTENT_MUSIC, m.GetIntent())
	require.Equal(t, "Extracted title", m.GetMusicTitle())
	require.Equal(t, "Extracted artist", m.GetMusicArtist())
	require.Equal(t, "Extracted album", m.GetMusicAlbum())
	require.EqualValues(t, 12, m.GetDurationSeconds())
	require.Equal(t, 1, extractor.calls)
	require.Empty(t, events.processed)
	require.EqualValues(t, 1, m.GetMusicMetadataRevision())
	files := store.NewFilesStore(pool)
	var first []byte
	_, err = files.DispatchMusicEvent(ctx, func(_ context.Context, body []byte) error {
		first = append([]byte(nil), body...)
		return errors.New("unconfirmed publish")
	})
	require.Error(t, err)
	published, err := files.DispatchMusicEvent(ctx, func(_ context.Context, body []byte) error {
		require.Equal(t, first, body, "retry must preserve event identity and revision")
		var event eventsv1.FileStreamEvent
		require.NoError(t, proto.Unmarshal(body, &event))
		require.Equal(t, chat.String(), event.GetFileProcessed().GetMusic().GetChatId())
		require.Equal(t, id, event.GetFileProcessed().GetFileId())
		require.EqualValues(t, 1, event.GetFileProcessed().GetMusic().GetMetadataRevision())
		return nil
	})
	require.NoError(t, err)
	require.True(t, published)
	published, err = files.DispatchMusicEvent(ctx, func(context.Context, []byte) error { t.Fatal("already published event replayed"); return nil })
	require.NoError(t, err)
	require.False(t, published)
	stored, err := client.GetFileMetadata(authed, &filev1.GetFileMetadataRequest{FileId: id})
	require.NoError(t, err)
	require.Equal(t, m.GetMusicTitle(), stored.GetFileMetadata().GetMusicTitle())
}

func TestUnspecifiedAudioPreservesLegacyProcessing(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	profile, chat := uuid.New(), uuid.New()
	guard := e2eGateGuard{members: map[uuid.UUID][]uuid.UUID{chat: {profile}}, e2eEnabled: map[uuid.UUID]bool{chat: false}, chatTypes: map[uuid.UUID]string{chat: "dm"}}
	extractor := &musicExtractorSpy{}
	reader := eventObjectReader{}
	client := dialEventTestGRPC(t, New(Deps{Files: store.NewFilesStore(pool), Presigner: eventTestPresigner{}, Reader: reader, ChatGuard: guard, Scanner: eventScanner("clean"), AudioProcessor: extractor}))
	authed := fileGateCtx(ctx, uuid.New(), profile)
	payload := []byte("legacy audio opaque to Music extractor")
	upload, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{OriginalName: "recording.m4a", MimeType: "audio/mp4", SizeBytes: int64(len(payload)), ContextChat: chatDMRefGate(chat)})
	require.NoError(t, err)
	reader[upload.GetUploadResponse().GetR2Key()] = payload
	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: upload.GetUploadResponse().GetFileId(), Sha256Hash: shaHex(payload)})
	require.NoError(t, err)
	require.Equal(t, "ready", confirmed.GetFileMetadata().GetStatus())
	require.Equal(t, filev1.UploadIntent_UPLOAD_INTENT_UNSPECIFIED, confirmed.GetFileMetadata().GetIntent())
	require.Zero(t, extractor.calls)
	require.Zero(t, confirmed.GetFileMetadata().GetMusicMetadataRevision())
	var events int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_music_event_outbox`).Scan(&events))
	require.Zero(t, events)
}

func TestMusicDeleteBeforeReadyRetainsTombstoneAndRejectsLateExtraction(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	files := store.NewFilesStore(pool)
	chat, id := uuid.New(), uuid.New()
	_, err := files.InsertPendingFile(ctx, store.FileRow{ID: id, UploaderProfileID: uuid.New(), OriginalName: "track.mp3", MimeType: "audio/mpeg", SizeBytes: 4, R2Key: id.String(), Status: "processing", FileType: "audio", UploadIntent: "music", ChatID: &chat, ScanResult: "skipped"})
	require.NoError(t, err)
	require.NoError(t, files.MarkDeleted(ctx, id))
	require.NoError(t, files.MarkDeleted(ctx, id))
	_, err = files.ApplyAudioProcessing(ctx, id, "late title", "", "", 1)
	require.Error(t, err)
	row, err := files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "deleted", row.Status)
	require.EqualValues(t, 1, row.MusicMetadataRevision)
	require.Nil(t, row.MusicTitle)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_music_event_outbox WHERE file_id=$1`, id).Scan(&count))
	require.Equal(t, 1, count)
	published, err := files.DispatchMusicEvent(ctx, func(_ context.Context, body []byte) error {
		var event eventsv1.FileStreamEvent
		require.NoError(t, proto.Unmarshal(body, &event))
		require.Equal(t, "deleted", event.GetFileProcessed().GetStatus())
		require.Empty(t, event.GetFileProcessed().GetMusic().GetTitle())
		require.EqualValues(t, 1, event.GetFileProcessed().GetMusic().GetMetadataRevision())
		return nil
	})
	require.NoError(t, err)
	require.True(t, published)
}

func TestEncryptedMusicNeverCallsServerExtractor(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	profile, chat := uuid.New(), uuid.New()
	guard := e2eGateGuard{members: map[uuid.UUID][]uuid.UUID{chat: {profile}}, e2eEnabled: map[uuid.UUID]bool{chat: true}, chatTypes: map[uuid.UUID]string{chat: "dm"}}
	extractor := &musicExtractorSpy{}
	reader := eventObjectReader{}
	client := dialEventTestGRPC(t, New(Deps{Files: store.NewFilesStore(pool), Presigner: eventTestPresigner{}, Reader: reader, ChatGuard: guard, AudioProcessor: extractor}))
	authed := fileGateCtx(ctx, uuid.New(), profile)
	encrypted := true
	payload := []byte("opaque ciphertext")
	upload, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{Intent: filev1.UploadIntent_UPLOAD_INTENT_MUSIC, OriginalName: "encrypted-audio", MimeType: "audio/mpeg", SizeBytes: int64(len(payload)), ContextChat: chatDMRefGate(chat), IsE2E: &encrypted})
	require.NoError(t, err)
	reader[upload.GetUploadResponse().GetR2Key()] = payload
	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: upload.GetUploadResponse().GetFileId(), Sha256Hash: shaHex(payload)})
	require.NoError(t, err)
	require.True(t, confirmed.GetFileMetadata().GetIsE2E())
	require.Equal(t, "ready", confirmed.GetFileMetadata().GetStatus())
	require.Zero(t, extractor.calls)
	require.Empty(t, confirmed.GetFileMetadata().GetMusicTitle())
	require.Empty(t, confirmed.GetFileMetadata().GetMusicArtist())
	require.Empty(t, confirmed.GetFileMetadata().GetMusicAlbum())
	require.Nil(t, confirmed.GetFileMetadata().DurationSeconds)
}
