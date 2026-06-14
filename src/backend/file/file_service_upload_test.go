package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/integrationtest"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
)

const freeFileLimitBytes = 50 << 20

var fixedNow = time.Date(2026, time.June, 3, 12, 0, 0, 0, time.UTC)

type recordingPresigner struct {
	putCalls []r2file.PutPresignInput
	getCalls []r2file.GetPresignInput
}

func (p *recordingPresigner) PresignPut(_ context.Context, in r2file.PutPresignInput) (string, error) {
	p.putCalls = append(p.putCalls, in)
	return "https://r2.example/upload/" + in.Key, nil
}

func (p *recordingPresigner) PresignGet(_ context.Context, in r2file.GetPresignInput) (string, error) {
	p.getCalls = append(p.getCalls, in)
	return "https://r2.example/download/" + in.Key, nil
}

type fixedClock struct{}

func (fixedClock) Now() time.Time {
	return fixedNow
}

type fakeImageProcessor struct{}

func (fakeImageProcessor) ProcessImage(_ context.Context, row store.FileRow) (grpcsvc.ImageProcessingResult, error) {
	return grpcsvc.ImageProcessingResult{
		ConvertedR2Key: "processed/" + row.ID.String() + "/full.webp",
		ThumbnailR2Key: "processed/" + row.ID.String() + "/thumb.webp",
		Width:          640,
		Height:         480,
	}, nil
}

type fakeObjectReader map[string][]byte

func (r fakeObjectReader) ReadObject(_ context.Context, key string, _ int64) ([]byte, error) {
	return r[key], nil
}

type fakeScanner string

func (s fakeScanner) ScanBytes(context.Context, []byte) (string, error) {
	return string(s), nil
}

func TestRequestUploadRequiresProfileMetadata(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})

	_, err := client.RequestUpload(ctx, validUploadRequest())
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestRequestUploadValidatesMetadataAndFreeLimit(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	authed := withFileProfile(ctx, uuid.New(), uuid.New())

	tests := []struct {
		name string
		req  *filev1.RequestUploadRequest
	}{
		{
			name: "original_name required",
			req: &filev1.RequestUploadRequest{
				MimeType:  "image/png",
				SizeBytes: 1024,
			},
		},
		{
			name: "mime_type required",
			req: &filev1.RequestUploadRequest{
				OriginalName: "cat.png",
				SizeBytes:    1024,
			},
		},
		{
			name: "positive size required",
			req: &filev1.RequestUploadRequest{
				OriginalName: "cat.png",
				MimeType:     "image/png",
			},
		},
		{
			name: "free tier limit enforced defensively",
			req: &filev1.RequestUploadRequest{
				OriginalName: "too-large.bin",
				MimeType:     "application/octet-stream",
				SizeBytes:    freeFileLimitBytes + 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.RequestUpload(authed, tt.req)
			require.Error(t, err)
			require.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}

func TestRequestUploadCreatesPendingFileAndPresignedPutURL(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	presigner := &recordingPresigner{}
	client := startFileGRPC(t, pool, presigner)
	profileID := uuid.New()

	resp, err := client.RequestUpload(withFileProfile(ctx, uuid.New(), profileID), &filev1.RequestUploadRequest{
		OriginalName: "diagram.png",
		MimeType:     "image/png",
		SizeBytes:    freeFileLimitBytes,
	})
	require.NoError(t, err)
	upload := resp.GetUploadResponse()
	require.NotNil(t, upload)
	fileID, err := uuid.Parse(upload.GetFileId())
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, fileID)
	require.Contains(t, upload.GetR2Key(), "attachments/")
	require.Equal(t, "https://r2.example/upload/"+upload.GetR2Key(), upload.GetPresignedPutUrl())
	require.Len(t, presigner.putCalls, 1)
	require.Equal(t, upload.GetR2Key(), presigner.putCalls[0].Key)
	require.Equal(t, "image/png", presigner.putCalls[0].ContentType)

	var stored struct {
		UploaderProfileID string
		OriginalName      string
		MimeType          string
		SizeBytes         int64
		R2Key             string
		Status            string
		ScanResult        string
		ChatIDIsNull      bool
	}
	err = pool.QueryRow(ctx, `
SELECT uploader_profile_id::text, original_name, mime_type, size_bytes, r2_key, status, scan_result, chat_id IS NULL
FROM files
WHERE id = $1
`, fileID).Scan(
		&stored.UploaderProfileID,
		&stored.OriginalName,
		&stored.MimeType,
		&stored.SizeBytes,
		&stored.R2Key,
		&stored.Status,
		&stored.ScanResult,
		&stored.ChatIDIsNull,
	)
	require.NoError(t, err)
	require.Equal(t, profileID.String(), stored.UploaderProfileID)
	require.Equal(t, "diagram.png", stored.OriginalName)
	require.Equal(t, "image/png", stored.MimeType)
	require.Equal(t, int64(freeFileLimitBytes), stored.SizeBytes)
	require.Equal(t, upload.GetR2Key(), stored.R2Key)
	require.Equal(t, "pending_upload", stored.Status)
	require.Equal(t, "pending", stored.ScanResult)
	require.True(t, stored.ChatIDIsNull)
}

func TestRequestUploadWithChatContextAllowsMembersAndStoresChat(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	ownerID := uuid.New()
	peerID := uuid.New()
	chatID := uuid.New()
	client := startFileGRPCWired(t, pool, &recordingPresigner{}, memberGuard{chatID: {ownerID, peerID}}, nil)
	req := validUploadRequest()
	req.ContextChat = chatDMRef(chatID)

	uploadResp, err := client.RequestUpload(withFileProfile(ctx, uuid.New(), ownerID), req)
	require.NoError(t, err)
	fileID := uploadResp.GetUploadResponse().GetFileId()
	_, err = client.ConfirmUpload(withFileProfile(ctx, uuid.New(), ownerID), &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	require.NoError(t, err)

	meta, err := client.GetFileMetadata(withFileProfile(ctx, uuid.New(), peerID), &filev1.GetFileMetadataRequest{FileId: fileID})
	require.NoError(t, err)
	require.Equal(t, chatID.String(), meta.GetFileMetadata().GetChat().GetId())

	_, err = client.GetFileURL(withFileProfile(ctx, uuid.New(), peerID), &filev1.GetFileURLRequest{FileId: fileID})
	require.NoError(t, err)

	_, err = client.GetFileURL(withFileProfile(ctx, uuid.New(), uuid.New()), &filev1.GetFileURLRequest{FileId: fileID})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

func TestGetFileURLUsesStoredR2KeyAndOneHourTTL(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	presigner := &recordingPresigner{}
	client := startFileGRPC(t, pool, presigner)
	fileID := uuid.New()
	profileID := uuid.New()
	r2Key := "attachments/" + fileID.String() + "/report.pdf"
	seedFile(t, ctx, pool, fileID, profileID, r2Key, "ready")

	before := fixedNow.Add(time.Hour)
	resp, err := client.GetFileURL(withFileProfile(ctx, uuid.New(), profileID), &filev1.GetFileURLRequest{
		FileId: fileID.String(),
	})
	require.NoError(t, err)
	require.Equal(t, "https://r2.example/download/"+r2Key, resp.GetPresignedGetUrl())
	require.WithinDuration(t, before, resp.GetExpiresAt().AsTime(), 5*time.Second)
	require.Len(t, presigner.getCalls, 1)
	require.Equal(t, r2Key, presigner.getCalls[0].Key)
	require.Equal(t, time.Hour, presigner.getCalls[0].TTL)
}

func TestGetFileURLRequiresUploaderProfileAndReadyStatus(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	ownerID := uuid.New()
	readyID := uuid.New()
	pendingID := uuid.New()
	seedFile(t, ctx, pool, readyID, ownerID, "attachments/"+readyID.String()+"/ready.pdf", "ready")
	seedFile(t, ctx, pool, pendingID, ownerID, "attachments/"+pendingID.String()+"/pending.pdf", "pending_upload")

	_, err := client.GetFileURL(withFileProfile(ctx, uuid.New(), uuid.New()), &filev1.GetFileURLRequest{
		FileId: readyID.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.GetFileURL(withFileProfile(ctx, uuid.New(), ownerID), &filev1.GetFileURLRequest{
		FileId: pendingID.String(),
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestConfirmUploadMetadataListQuotaAndDelete(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	profileID := uuid.New()
	authed := withFileProfile(ctx, uuid.New(), profileID)

	uploadResp, err := client.RequestUpload(authed, validUploadRequest())
	require.NoError(t, err)
	fileID := uploadResp.GetUploadResponse().GetFileId()

	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	require.NoError(t, err)
	meta := confirmed.GetFileMetadata()
	require.Equal(t, fileID, meta.GetId())
	require.Equal(t, profileID.String(), meta.GetUploaderProfileId())
	require.Equal(t, "ready", meta.GetStatus())
	require.Equal(t, filev1.FileLifecycleStatus_FILE_LIFECYCLE_STATUS_READY, meta.GetStatusEnum())
	require.Equal(t, "skipped", meta.GetScanResult())
	require.Equal(t, "cat.png", meta.GetOriginalName())

	getMeta, err := client.GetFileMetadata(authed, &filev1.GetFileMetadataRequest{FileId: fileID})
	require.NoError(t, err)
	require.Equal(t, fileID, getMeta.GetFileMetadata().GetId())

	bulk, err := client.GetBulkMetadata(authed, &filev1.GetBulkMetadataRequest{FileIds: []string{fileID, uuid.New().String()}})
	require.NoError(t, err)
	require.Contains(t, bulk.GetBulkFileMetadata().GetByFileId(), fileID)
	require.Len(t, bulk.GetBulkFileMetadata().GetByFileId(), 1)

	listed, err := client.ListFiles(authed, &filev1.ListFilesRequest{})
	require.NoError(t, err)
	require.Len(t, listed.GetFileList().GetFiles(), 1)
	require.Equal(t, fileID, listed.GetFileList().GetFiles()[0].GetId())

	quota, err := client.CheckQuota(authed, &filev1.CheckQuotaRequest{})
	require.NoError(t, err)
	require.Equal(t, int64(1024), quota.GetQuotaResponse().GetBytesUsed())
	require.Equal(t, int64(freeFileLimitBytes), quota.GetQuotaResponse().GetBytesLimit())

	_, err = client.DeleteFile(authed, &filev1.DeleteFileRequest{FileId: fileID})
	require.NoError(t, err)
	_, err = client.GetFileURL(authed, &filev1.GetFileURLRequest{FileId: fileID})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestConfirmUploadProcessesImageMetadata(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPCWired(t, pool, &recordingPresigner{}, nil, fakeImageProcessor{})
	profileID := uuid.New()
	authed := withFileProfile(ctx, uuid.New(), profileID)
	uploadResp, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "screenshot.png",
		MimeType:     "image/png",
		SizeBytes:    4096,
	})
	require.NoError(t, err)

	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{
		FileId:     uploadResp.GetUploadResponse().GetFileId(),
		Sha256Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	})
	require.NoError(t, err)
	meta := confirmed.GetFileMetadata()
	require.Equal(t, "ready", meta.GetStatus())
	require.Equal(t, "processed/"+meta.GetId()+"/full.webp", meta.GetConvertedR2Key())
	require.Equal(t, "processed/"+meta.GetId()+"/thumb.webp", meta.GetThumbnailR2Key())
	require.Equal(t, int32(640), meta.GetWidth())
	require.Equal(t, int32(480), meta.GetHeight())
}

func TestConfirmUploadRejectsWrongOwnerAndInvalidHash(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	client := startFileGRPC(t, pool, &recordingPresigner{})
	ownerID := uuid.New()
	uploadResp, err := client.RequestUpload(withFileProfile(ctx, uuid.New(), ownerID), validUploadRequest())
	require.NoError(t, err)
	fileID := uploadResp.GetUploadResponse().GetFileId()

	_, err = client.ConfirmUpload(withFileProfile(ctx, uuid.New(), uuid.New()), &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = client.ConfirmUpload(withFileProfile(ctx, uuid.New(), ownerID), &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: "not-a-sha",
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestConfirmUploadScansRiskyFilesAndBlocksInfectedDownload(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	presigner := &recordingPresigner{}
	profileID := uuid.New()
	client := startFileGRPCFull(t, pool, grpcsvc.Deps{
		Presigner: presigner,
		Reader:    fakeObjectReader{},
		Scanner:   fakeScanner("infected"),
	})
	authed := withFileProfile(ctx, uuid.New(), profileID)
	uploadResp, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{
		OriginalName: "payload.zip",
		MimeType:     "application/zip",
		SizeBytes:    1024,
	})
	require.NoError(t, err)
	fileID := uploadResp.GetUploadResponse().GetFileId()
	presignedKey := uploadResp.GetUploadResponse().GetR2Key()
	client = startFileGRPCFull(t, pool, grpcsvc.Deps{
		Presigner: presigner,
		Reader:    fakeObjectReader{presignedKey: []byte("zip")},
		Scanner:   fakeScanner("infected"),
	})

	confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{
		FileId:     fileID,
		Sha256Hash: "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	})
	require.NoError(t, err)
	require.Equal(t, "failed", confirmed.GetFileMetadata().GetStatus())
	require.Equal(t, "infected", confirmed.GetFileMetadata().GetScanResult())

	_, err = client.GetFileURL(authed, &filev1.GetFileURLRequest{FileId: fileID})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func validUploadRequest() *filev1.RequestUploadRequest {
	return &filev1.RequestUploadRequest{
		OriginalName: "cat.png",
		MimeType:     "image/png",
		SizeBytes:    1024,
	}
}

func chatDMRef(chatID uuid.UUID) *chatv1.ChatRef {
	dm := chatv1.ChatType_CHAT_TYPE_DM
	return &chatv1.ChatRef{Id: chatID.String(), Type: &dm}
}

func withFileProfile(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-user-id", accountID.String())
	return metadata.AppendToOutgoingContext(ctx, "x-voice-profile-id", profileID.String())
}

func startFilePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "db", "")
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000001_init.up.sql"))
	applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000002_premium_upload_limit.up.sql"))
	return pool
}

func applyFileSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, relPath string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fileRepoRoot(t), relPath))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)
}

func fileRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", ".."))
}

func startFileGRPC(t *testing.T, pool *pgxpool.Pool, presigner *recordingPresigner) filev1.FileServiceClient {
	t.Helper()
	return startFileGRPCWired(t, pool, presigner, nil, nil)
}

func startFileGRPCWired(t *testing.T, pool *pgxpool.Pool, presigner *recordingPresigner, chatGuard grpcsvc.ChatGuard, processor grpcsvc.ImageProcessor) filev1.FileServiceClient {
	t.Helper()
	return startFileGRPCFull(t, pool, grpcsvc.Deps{
		Presigner: presigner,
		ChatGuard: chatGuard,
		Processor: processor,
	})
}

func startFileGRPCFull(t *testing.T, pool *pgxpool.Pool, deps grpcsvc.Deps) filev1.FileServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	deps.Files = store.NewFilesStore(pool)
	deps.Clock = fixedClock{}
	filev1.RegisterFileServiceServer(srv, grpcsvc.New(grpcsvc.Deps{
		Files:     deps.Files,
		Presigner: deps.Presigner,
		Clock:     deps.Clock,
		ChatGuard: deps.ChatGuard,
		Processor: deps.Processor,
		Reader:    deps.Reader,
		Scanner:   deps.Scanner,
	}))
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

type memberGuard map[uuid.UUID][]uuid.UUID

func (g memberGuard) EnsureMember(_ context.Context, chatID, profileID uuid.UUID) error {
	for _, member := range g[chatID] {
		if member == profileID {
			return nil
		}
	}
	return grpcsvc.ErrNotChatMember
}

func seedFile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, fileID, profileID uuid.UUID, r2Key, status string) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO files (
	id,
	uploader_profile_id,
	original_name,
	mime_type,
	size_bytes,
	r2_key,
	status,
	file_type,
	scan_result
) VALUES ($1, $2, 'report.pdf', 'application/pdf', 4096, $3, $4, 'document', 'clean')
`, fileID, profileID, r2Key, status)
	require.NoError(t, err)
}
