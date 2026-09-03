package grpcsvc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/file/internal/fileevents"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
)

type spyFileEvents struct {
	uploaded      []string
	infected      []string
	processed     []string
	downloaded    []downloadedFileEvent
	downloadedErr error
}

type downloadedFileEvent struct {
	fileID              string
	downloaderProfileID string
}

func (s *spyFileEvents) PublishFileUploaded(_ context.Context, fileID, _ string) error {
	s.uploaded = append(s.uploaded, fileID)
	return nil
}

func (s *spyFileEvents) PublishFileScanInfected(_ context.Context, fileID, _ string) error {
	s.infected = append(s.infected, fileID)
	return nil
}

func (s *spyFileEvents) PublishFileProcessed(_ context.Context, fileID, _, _, _ string) error {
	s.processed = append(s.processed, fileID)
	return nil
}

func (s *spyFileEvents) PublishFileExpired(context.Context, string, *string) error { return nil }
func (s *spyFileEvents) PublishFileDownloaded(_ context.Context, fileID, downloaderProfileID string) error {
	s.downloaded = append(s.downloaded, downloadedFileEvent{
		fileID:              fileID,
		downloaderProfileID: downloaderProfileID,
	})
	return s.downloadedErr
}
func (s *spyFileEvents) Close() error { return nil }

type eventTestPresigner struct{}

func (eventTestPresigner) PresignPut(context.Context, r2file.PutPresignInput) (string, error) {
	return "https://r2.example/upload", nil
}

func (eventTestPresigner) PresignGet(context.Context, r2file.GetPresignInput) (string, error) {
	return "https://r2.example/download", nil
}

type downloadEventPresigner struct {
	getURL string
	getErr error
}

func (downloadEventPresigner) PresignPut(context.Context, r2file.PutPresignInput) (string, error) {
	return "https://r2.example/upload", nil
}

func (p downloadEventPresigner) PresignGet(context.Context, r2file.GetPresignInput) (string, error) {
	if p.getErr != nil {
		return "", p.getErr
	}
	return p.getURL, nil
}

type eventObjectReader map[string][]byte

func (r eventObjectReader) ReadObject(_ context.Context, key string, _ int64) ([]byte, error) {
	return r[key], nil
}

type eventScanner string

func (s eventScanner) ScanBytes(context.Context, []byte) (string, error) {
	return string(s), nil
}

type eventProcessor struct{}

func (eventProcessor) ProcessImage(_ context.Context, row store.FileRow) (ImageProcessingResult, error) {
	return ImageProcessingResult{
		ConvertedR2Key: "processed/" + row.ID.String() + "/full.webp",
		ThumbnailR2Key: "processed/" + row.ID.String() + "/thumb.webp",
		Width:          100,
		Height:         100,
	}, nil
}

func shaHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestConfirmUpload_PublishesInfectedEvent(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	events := &spyFileEvents{}
	payload := []byte("zip")
	svc := New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: eventTestPresigner{},
		Reader:    eventObjectReader{},
		Scanner:   eventScanner("infected"),
		Events:    events,
	})
	client := dialEventTestGRPC(t, svc)
	profileID := uuid.New()
	authed := fileGateCtx(ctx, uuid.New(), profileID)

	uploadResp, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "payload.zip",
		MimeType:     "application/zip",
		SizeBytes:    int64(len(payload)),
	})
	require.NoError(t, err)
	fileID := uploadResp.GetUploadResponse().GetFileId()
	key := uploadResp.GetUploadResponse().GetR2Key()

	svc = New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: eventTestPresigner{},
		Reader:    eventObjectReader{key: payload},
		Scanner:   eventScanner("infected"),
		Events:    events,
	})
	client = dialEventTestGRPC(t, svc)

	_, err = client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: shaHex(payload),
	})
	require.NoError(t, err)
	require.Equal(t, []string{fileID}, events.infected)
	require.Empty(t, events.uploaded)
}

func TestConfirmUpload_PublishesUploadedAndProcessedForImage(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	events := &spyFileEvents{}
	payload := []byte("png-bytes")
	svc := New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: eventTestPresigner{},
		Reader:    eventObjectReader{},
		Processor: eventProcessor{},
		Events:    events,
	})
	client := dialEventTestGRPC(t, svc)
	profileID := uuid.New()
	authed := fileGateCtx(ctx, uuid.New(), profileID)

	uploadResp, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "shot.png",
		MimeType:     "image/png",
		SizeBytes:    int64(len(payload)),
	})
	require.NoError(t, err)
	fileID := uploadResp.GetUploadResponse().GetFileId()
	key := uploadResp.GetUploadResponse().GetR2Key()

	svc = New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: eventTestPresigner{},
		Reader:    eventObjectReader{key: payload},
		Processor: eventProcessor{},
		Events:    events,
	})
	client = dialEventTestGRPC(t, svc)

	_, err = client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: shaHex(payload),
	})
	require.NoError(t, err)
	require.Equal(t, []string{fileID}, events.uploaded)
	require.Equal(t, []string{fileID}, events.processed)
}

func TestGetFileURL_PublishesDownloadedOnceAfterSuccessfulPresign(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	events := &spyFileEvents{}
	profileID := uuid.New()
	fileID := uuid.New()
	seedDownloadEventFile(t, ctx, pool, fileID, profileID, "ready")

	client := dialEventTestGRPC(t, New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: downloadEventPresigner{getURL: "https://r2.example/download-ready"},
		Events:    events,
	}))
	resp, err := client.GetFileURL(fileGateCtx(ctx, uuid.New(), profileID), &filev1.GetFileURLRequest{
		FileId: fileID.String(),
	})

	require.NoError(t, err)
	require.Equal(t, "https://r2.example/download-ready", resp.GetPresignedGetUrl())
	require.Equal(t, []downloadedFileEvent{{
		fileID:              fileID.String(),
		downloaderProfileID: profileID.String(),
	}}, events.downloaded)
}

func TestGetFileURL_DoesNotPublishDownloadedOnRejectedOrFailedRequest(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		callerProfile *uuid.UUID
		presignErr    error
		wantCode      codes.Code
	}{
		{
			name:     "missing authentication",
			status:   "ready",
			wantCode: codes.Unauthenticated,
		},
		{
			name:          "access denied",
			status:        "ready",
			callerProfile: uuidPtr(uuid.New()),
			wantCode:      codes.PermissionDenied,
		},
		{
			name:          "file is not ready",
			status:        "processing",
			callerProfile: nil,
			wantCode:      codes.FailedPrecondition,
		},
		{
			name:          "presigner fails",
			status:        "ready",
			callerProfile: nil,
			presignErr:    errors.New("r2 unavailable"),
			wantCode:      codes.Internal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			pool := startFileGatePostgres(t, ctx)
			events := &spyFileEvents{}
			ownerProfileID := uuid.New()
			fileID := uuid.New()
			seedDownloadEventFile(t, ctx, pool, fileID, ownerProfileID, tt.status)

			client := dialEventTestGRPC(t, New(Deps{
				Files:     store.NewFilesStore(pool),
				Presigner: downloadEventPresigner{getURL: "https://r2.example/download", getErr: tt.presignErr},
				Events:    events,
			}))
			requestCtx := ctx
			if tt.name != "missing authentication" {
				callerProfileID := ownerProfileID
				if tt.callerProfile != nil {
					callerProfileID = *tt.callerProfile
				}
				requestCtx = fileGateCtx(ctx, uuid.New(), callerProfileID)
			}

			_, err := client.GetFileURL(requestCtx, &filev1.GetFileURLRequest{FileId: fileID.String()})

			require.Equal(t, tt.wantCode, status.Code(err))
			require.Empty(t, events.downloaded)
		})
	}
}

func TestGetFileURL_DownloadedPublisherFailureIsBestEffort(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	events := &spyFileEvents{downloadedErr: errors.New("nats unavailable")}
	profileID := uuid.New()
	fileID := uuid.New()
	seedDownloadEventFile(t, ctx, pool, fileID, profileID, "ready")

	client := dialEventTestGRPC(t, New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: downloadEventPresigner{getURL: "https://r2.example/download-best-effort"},
		Events:    events,
	}))
	resp, err := client.GetFileURL(fileGateCtx(ctx, uuid.New(), profileID), &filev1.GetFileURLRequest{
		FileId: fileID.String(),
	})

	require.NoError(t, err)
	require.Equal(t, "https://r2.example/download-best-effort", resp.GetPresignedGetUrl())
	require.Equal(t, []downloadedFileEvent{{
		fileID:              fileID.String(),
		downloaderProfileID: profileID.String(),
	}}, events.downloaded)
}

func TestCheckQuota_PremiumLimit(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	svc := New(Deps{Files: store.NewFilesStore(pool)})
	client := dialEventTestGRPC(t, svc)
	profileID := uuid.New()
	authed := metadata.AppendToOutgoingContext(
		fileGateCtx(ctx, uuid.New(), profileID),
		"x-voice-subscription-tier", "premium",
	)

	resp, err := client.CheckQuota(authed, &filev1.CheckQuotaRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(r2file.MaxPremiumFileBytes), resp.GetQuotaResponse().GetBytesLimit())
}

func TestListFiles_FilterByChat(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	chatID := uuid.New()
	profileID := uuid.New()
	otherChat := uuid.New()
	guard := listChatGuard{
		members: map[uuid.UUID][]uuid.UUID{
			chatID:    {profileID},
			otherChat: {profileID},
		},
	}

	svc := New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: eventTestPresigner{},
		ChatGuard: guard,
	})
	client := dialEventTestGRPC(t, svc)
	authed := fileGateCtx(ctx, uuid.New(), profileID)

	inChat := uuid.New()
	otherFile := uuid.New()
	seedListChatFile(t, ctx, pool, inChat, profileID, chatID, "dm")
	seedListChatFile(t, ctx, pool, otherFile, profileID, otherChat, "dm")

	listed, err := client.ListFiles(authed, &filev1.ListFilesRequest{
		FilterChat: listChatDMRef(chatID),
	})
	require.NoError(t, err)
	require.Len(t, listed.GetFileList().GetFiles(), 1)
	require.Equal(t, inChat.String(), listed.GetFileList().GetFiles()[0].GetId())
}

type listChatGuard struct {
	members map[uuid.UUID][]uuid.UUID
}

func (g listChatGuard) EnsureMember(_ context.Context, chatID, profileID uuid.UUID) error {
	for _, member := range g.members[chatID] {
		if member == profileID {
			return nil
		}
	}
	return ErrNotChatMember
}

func (g listChatGuard) ChatE2EState(_ context.Context, _ uuid.UUID) (string, bool, error) {
	return "dm", false, nil
}

func listChatDMRef(chatID uuid.UUID) *chatv1.ChatRef {
	dm := chatv1.ChatType_CHAT_TYPE_DM
	return &chatv1.ChatRef{Id: chatID.String(), Type: &dm}
}

func seedListChatFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID, profileID, chatID uuid.UUID, chatType string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO files (
	id, uploader_profile_id, original_name, mime_type, size_bytes, r2_key,
	status, file_type, chat_id, chat_type, scan_result
) VALUES ($1, $2, 'doc.pdf', 'application/pdf', 1024, $3, 'ready', 'document', $4, $5, 'clean')
`, fileID, profileID, "attachments/"+fileID.String()+"/doc.pdf", chatID, chatType)
	require.NoError(t, err)
}

func seedDownloadEventFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID, profileID uuid.UUID, fileStatus string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO files (
	id, uploader_profile_id, original_name, mime_type, size_bytes, r2_key,
	status, file_type, scan_result
) VALUES ($1, $2, 'download.pdf', 'application/pdf', 1024, $3, $4, 'document', 'clean')
`, fileID, profileID, "attachments/"+fileID.String()+"/download.pdf", fileStatus)
	require.NoError(t, err)
}

func uuidPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

func dialEventTestGRPC(t *testing.T, svc *FileGRPC) filev1.FileServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	filev1.RegisterFileServiceServer(srv, svc)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return filev1.NewFileServiceClient(conn)
}

var _ fileevents.Publisher = (*spyFileEvents)(nil)
