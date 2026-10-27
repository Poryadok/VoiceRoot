package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	filev1 "voice.app/voice/file/v1"
	storyv1 "voice.app/voice/story/v1"
	"voice/backend/file/internal/grpcsvc"
	"voice/backend/file/internal/imgproc"
)

type stickerObjects map[string][]byte

func (s stickerObjects) ReadObject(_ context.Context, key string, _ int64) ([]byte, error) {
	return s[key], nil
}
func (s stickerObjects) PutObject(_ context.Context, key, _ string, data []byte) error {
	s[key] = data
	return nil
}

func TestStaticStickerUpload(t *testing.T) {
	ctx := context.Background()
	pool := startFilePostgres(t, ctx)
	for _, migration := range []string{"000009_gif_processing", "000010_catalog_asset_import", "000011_tracked_write_owner_kind", "000012_catalog_selection_authority", "000013_static_sticker_intent"} {
		applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", migration+".up.sql"))
	}
	objects := stickerObjects{}
	client := startFileGRPCFull(t, pool, grpcsvc.Deps{Presigner: &recordingPresigner{}, Reader: objects, Processor: imgproc.Processor{Reader: objects, Writer: objects}, Scanner: fakeScanner("clean")})
	authed := withFileProfile(ctx, uuid.New(), uuid.New())
	var source bytes.Buffer
	sourceImage := image.NewRGBA(image.Rect(0, 0, 1024, 256))
	draw.Draw(sourceImage, sourceImage.Bounds(), image.NewUniform(color.RGBA{R: 255, A: 255}), image.Point{}, draw.Src)
	require.NoError(t, png.Encode(&source, sourceImage))
	request := func() *filev1.RequestUploadRequest {
		return &filev1.RequestUploadRequest{OriginalName: "sticker.png", MimeType: "image/png", SizeBytes: int64(source.Len()), Intent: filev1.UploadIntent(3)}
	}
	for _, mime := range []string{"image/png", "image/webp"} {
		t.Run(mime, func(t *testing.T) {
			req := request()
			req.MimeType = mime
			body := source.Bytes()
			if mime == "image/webp" {
				for key, value := range objects {
					if len(key) > 10 && key[len(key)-10:] == "/full.webp" {
						body = value
						break
					}
				}
				req.OriginalName = "sticker.webp"
				req.SizeBytes = int64(len(body))
			}
			upload, err := client.RequestUpload(authed, req)
			require.NoError(t, err)
			id := upload.GetUploadResponse().GetFileId()
			var intent string
			require.NoError(t, pool.QueryRow(ctx, "SELECT upload_intent FROM files WHERE id=$1", id).Scan(&intent))
			require.Equal(t, "sticker", intent)
			objects[upload.GetUploadResponse().GetR2Key()] = body
			confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: id, Sha256Hash: sha256Hex(body)})
			require.NoError(t, err)
			require.Equal(t, "ready", confirmed.GetFileMetadata().GetStatus())
			require.Equal(t, filev1.UploadIntent_UPLOAD_INTENT_STICKER, confirmed.GetFileMetadata().GetIntent())
			var key, scan string
			var width, height int
			require.NoError(t, pool.QueryRow(ctx, "SELECT converted_r2_key, width, height, scan_result FROM files WHERE id=$1", id).Scan(&key, &width, &height, &scan))
			require.Equal(t, 512, width)
			require.Equal(t, 512, height)
			require.Equal(t, "clean", scan)
			img, format, err := image.Decode(bytes.NewReader(objects[key]))
			require.NoError(t, err)
			require.Equal(t, "webp", format)
			require.Equal(t, image.Rect(0, 0, 512, 512), img.Bounds())
			_, _, _, alpha := img.At(0, 0).RGBA()
			require.Zero(t, alpha)
			_, _, _, alpha = img.At(256, 256).RGBA()
			require.Equal(t, uint32(65535), alpha)
		})
	}
	for _, mime := range []string{"image/gif", "image/jpeg", "application/octet-stream"} {
		req := request()
		req.MimeType = mime
		_, err := client.RequestUpload(authed, req)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	}
	req := request()
	encrypted := true
	req.IsE2E = &encrypted
	_, err := client.RequestUpload(authed, req)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	req = request()
	req.ContextStory = &storyv1.StoryRef{StoryId: uuid.NewString()}
	_, err = client.RequestUpload(authed, req)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	// A MIME declaration cannot turn an animated or GIF source into a static sticker.
	animatedPNG := append([]byte(nil), source.Bytes()[:33]...)
	animationChunk := make([]byte, 20)
	binary.BigEndian.PutUint32(animationChunk, 8)
	copy(animationChunk[4:], "acTL")
	animatedPNG = append(animatedPNG, animationChunk...)
	animatedPNG = append(animatedPNG, source.Bytes()[33:]...)
	for _, source := range []struct {
		mime string
		body []byte
	}{
		{"image/png", []byte("GIF89a")},
		{"image/png", animatedPNG},
		{"image/webp", append([]byte("RIFF\x12\x00\x00\x00WEBPANIM\x06\x00\x00\x00"), make([]byte, 6)...)},
	} {
		req := request()
		req.MimeType = source.mime
		req.SizeBytes = int64(len(source.body))
		upload, err := client.RequestUpload(authed, req)
		require.NoError(t, err)
		objects[upload.GetUploadResponse().GetR2Key()] = source.body
		confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: upload.GetUploadResponse().GetFileId(), Sha256Hash: sha256Hex(source.body)})
		require.NoError(t, err)
		require.Equal(t, "failed", confirmed.GetFileMetadata().GetStatus())
	}
	for _, outcome := range []string{"infected", "error"} {
		client := startFileGRPCFull(t, pool, grpcsvc.Deps{Presigner: &recordingPresigner{}, Reader: objects, Processor: imgproc.Processor{Reader: objects, Writer: objects}, Scanner: fakeScanner(outcome)})
		upload, err := client.RequestUpload(authed, request())
		require.NoError(t, err)
		objects[upload.GetUploadResponse().GetR2Key()] = source.Bytes()
		confirmed, err := client.ConfirmUpload(authed, &filev1.ConfirmUploadRequest{FileId: upload.GetUploadResponse().GetFileId(), Sha256Hash: sha256Hex(source.Bytes())})
		require.NoError(t, err)
		require.Equal(t, "failed", confirmed.GetFileMetadata().GetStatus())
		require.Empty(t, confirmed.GetFileMetadata().GetConvertedR2Key())
	}
	var previews int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM file_preview_event_outbox").Scan(&previews))
	require.Zero(t, previews, "stickers do not enter the closed generic preview protocol")
}
