package grpcsvc

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/store"
)

func TestGIFAdmissionIsExplicitAndPlaintext(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	client := dialEventTestGRPC(t, New(Deps{Files: store.NewFilesStore(pool), Presigner: eventTestPresigner{}}))
	authed := fileGateCtx(ctx, uuid.New(), uuid.New())
	request := &filev1.RequestUploadRequest{Intent: filev1.UploadIntent(2), OriginalName: "animation.gif", MimeType: "image/gif", SizeBytes: 1024}
	_, err := client.RequestUpload(authed, request)
	require.NoError(t, err, "explicit GIF API intent must be admitted")
	encrypted := true
	request.IsE2E = &encrypted
	_, err = client.RequestUpload(authed, request)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "GIF processing must never admit ciphertext")
	request.IsE2E = nil
	request.MimeType = "image/png"
	_, err = client.RequestUpload(authed, request)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	request.MimeType = "image/gif"
	request.Intent = filev1.UploadIntent_UPLOAD_INTENT_UNSPECIFIED
	_, err = client.RequestUpload(authed, request)
	require.Equal(t, codes.InvalidArgument, status.Code(err), "legacy MIME branch must never flatten a GIF")
	request.MimeType = "application/octet-stream"
	_, err = client.RequestUpload(authed, request)
	require.NoError(t, err, "A1 attachment fallback must remain opaque")
}

func TestGIFMissingRepresentationNeverFallsBack(t *testing.T) {
	row := store.FileRow{UploadIntent: "gif", Status: "ready", R2Key: "source/animation.gif"}
	_, err := fileURLKey(row, filev1.FileURLVariant_FILE_URL_VARIANT_UNSPECIFIED)
	require.Equal(t, codes.FailedPrecondition, status.Code(err), "missing GIF output must never expose original bytes")
}

func TestGIFConfirmationQueuesOnceAndReplays(t *testing.T) {
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	reader := eventObjectReader{}
	files := store.NewFilesStore(pool)
	client := dialEventTestGRPC(t, New(Deps{Files: files, Presigner: eventTestPresigner{}, Reader: reader}))
	authed := fileGateCtx(ctx, uuid.New(), uuid.New())
	raw := []byte("GIF89a fixture validated by codec in worker")
	ticket, err := client.RequestUpload(authed, &filev1.RequestUploadRequest{Intent: filev1.UploadIntent(2), OriginalName: "animation.gif", MimeType: "image/gif", SizeBytes: int64(len(raw))})
	require.NoError(t, err)
	reader[ticket.GetUploadResponse().GetR2Key()] = raw
	request := &filev1.ConfirmUploadRequest{FileId: ticket.GetUploadResponse().GetFileId(), Sha256Hash: shaHex(raw)}
	confirmed, err := client.ConfirmUpload(authed, request)
	require.NoError(t, err)
	require.Equal(t, "processing", confirmed.GetFileMetadata().GetStatus(), "confirmation must queue recoverable processing instead of static conversion")
	replayed, err := client.ConfirmUpload(authed, request)
	require.NoError(t, err)
	require.Equal(t, confirmed.GetFileMetadata(), replayed.GetFileMetadata())
	var jobs int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM file_gif_jobs WHERE file_id=$1`, request.FileId).Scan(&jobs))
	require.Equal(t, 1, jobs)
	request.Sha256Hash = shaHex([]byte("different"))
	_, err = client.ConfirmUpload(authed, request)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
