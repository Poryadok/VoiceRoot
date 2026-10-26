package grpcsvc

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	"voice/backend/file/internal/catalogasset"
	"voice/backend/file/internal/gifproc"
	"voice/backend/file/internal/jobs"
	"voice/backend/file/internal/r2file"
)

func catalogNativeConverter(t *testing.T) func(context.Context, []byte) (gifproc.Output, error) {
	t.Helper()
	_, ffmpegErr := exec.LookPath("ffmpeg")
	_, ffprobeErr := exec.LookPath("ffprobe")
	if ffmpegErr == nil && ffprobeErr == nil {
		return gifproc.Convert
	}
	imageID := os.Getenv("VOICE_TEST_CATALOG_CODEC_IMAGE")
	helper := os.Getenv("VOICE_TEST_CATALOG_CODEC_HELPER")
	if imageID == "" || helper == "" {
		t.Skip("native ffmpeg/ffprobe or explicit isolated engineering codec helper required")
	}
	require.True(t, strings.HasPrefix(imageID, "sha256:"))
	digest, err := hex.DecodeString(strings.TrimPrefix(imageID, "sha256:"))
	require.NoError(t, err)
	require.Len(t, digest, 32)
	require.True(t, filepath.IsAbs(helper))
	info, err := os.Stat(helper)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.Less(t, info.Size(), int64(50<<20))
	binary, err := os.ReadFile(helper)
	require.NoError(t, err)
	hash := sha256.Sum256(binary)
	t.Logf("isolated codec image=%s helper_sha256=%x", imageID, hash)
	return func(ctx context.Context, raw []byte) (gifproc.Output, error) {
		ctx, cancel := context.WithTimeout(ctx, 110*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", "run", "--rm", "-i", "--network", "none", "--read-only", "--cpus", "1", "--memory", "512m", "--user", "65534:65534", "--tmpfs", "/tmp:rw,nosuid,nodev,size=128m,mode=1777", "--mount", "type=bind,source="+helper+",target=/catalog-codec,readonly", "--entrypoint", "/catalog-codec", imageID)
		cmd.Stdin = bytes.NewReader(raw)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		encoded, e := cmd.Output()
		if e != nil {
			return gifproc.Output{}, fmt.Errorf("native codec runner: %w: %s", e, stderr.String())
		}
		var result gifproc.Output
		e = json.Unmarshal(encoded, &result)
		return result, e
	}
}

func TestCatalogImportActualBytesReachNativeReadyAuthority(t *testing.T) {
	if testing.Short() {
		t.Skip("actual PostgreSQL/object/native codec integration")
	}
	convert := catalogNativeConverter(t)
	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()
	f := seedCatalogInput(t, ctx)
	palette := color.Palette{color.Black, color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}}
	frame1 := image.NewPaletted(image.Rect(0, 0, 16, 16), palette)
	frame2 := image.NewPaletted(frame1.Rect, palette)
	for i := range frame1.Pix {
		frame1.Pix[i] = 1
		frame2.Pix[i] = 2
	}
	var source bytes.Buffer
	require.NoError(t, gif.EncodeAll(&source, &gif.GIF{Image: []*image.Paletted{frame1, frame2}, Delay: []int{10, 20}}))
	body := source.Bytes()
	type object struct {
		body []byte
		mime string
	}
	objects := map[string]object{}
	var mu sync.Mutex
	r2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		key := strings.TrimPrefix(r.URL.Path, "/bucket/")
		switch r.Method {
		case http.MethodPut:
			b, e := io.ReadAll(r.Body)
			if e != nil {
				http.Error(w, "read failed", 400)
				return
			}
			objects[key] = object{body: b, mime: r.Header.Get("Content-Type")}
			w.Header().Set("ETag", "fixture")
			w.WriteHeader(200)
		case http.MethodGet:
			o, ok := objects[key]
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", o.mime)
			w.WriteHeader(200)
			_, _ = w.Write(o.body)
		case http.MethodDelete:
			delete(objects, key)
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}))
	defer r2.Close()
	storage, err := r2file.NewS3R2Presigner(r2file.S3R2Config{Endpoint: r2.URL, Region: "auto", AccessKeyID: "fixture", SecretAccessKey: "fixture", Bucket: "bucket"})
	require.NoError(t, err)
	server := &CatalogAssetGRPC{Files: f.files, Keys: f.keys, Actors: catalogActorSnapshotFunc(func(context.Context, []catalogasset.ActorPair) ([]catalogasset.ActorState, error) {
		return []catalogasset.ActorState{catalogasset.ActorLive}, nil
	}), Destination: catalogDestinationFunc(func(context.Context, catalogasset.DestinationRequest) (chatv1.ChatType, error) {
		return chatv1.ChatType_CHAT_TYPE_GROUP, nil
	}), Fetcher: catalogFetchFunc(func(_ context.Context, p, u string, max int64) ([]byte, error) {
		require.Equal(t, f.request.Provider, p)
		require.Equal(t, f.request.SourceUrl, u)
		require.GreaterOrEqual(t, max, int64(len(body)))
		return body, nil
	}), Writer: storage}
	imported, err := server.ImportCatalogAsset(catalogVerifiedContext(t, f.request, filev1.CatalogAssetService_ImportCatalogAsset_FullMethodName, "chat"), f.request)
	require.NoError(t, err)
	require.Equal(t, "processing", imported.Status)
	processed, err := jobs.RunGIFProcessingOnce(ctx, f.files, storage, convert)
	require.NoError(t, err)
	require.True(t, processed)
	id, err := uuid.Parse(imported.FileId)
	require.NoError(t, err)
	row, err := f.files.GetFileByID(ctx, id)
	require.NoError(t, err)
	require.Equal(t, "ready", row.Status)
	require.NotNil(t, row.DeliveredFull)
	require.NotNil(t, row.DeliveredThumbnail)
	lookup := &filev1.LookupCatalogAssetRequest{ProtocolVersion: 2, ActorAccountId: f.actor.AccountID.String(), ActorProfileId: f.actor.ProfileID.String(), FileId: imported.FileId}
	attested, err := server.LookupCatalogAsset(catalogVerifiedContext(t, lookup, filev1.CatalogAssetService_LookupCatalogAsset_FullMethodName, "messaging"), lookup)
	require.NoError(t, err)
	snapshot := attested.GetCatalog()
	require.NotNil(t, snapshot)
	require.Equal(t, "gif", snapshot.Intent)
	require.Equal(t, "video", snapshot.FileType)
	sourceHash := sha256.Sum256(body)
	require.Equal(t, sourceHash[:], snapshot.SourceSha256)
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, body, objects[row.R2Key].body)
	full := objects[row.DeliveredFull.Key].body
	thumb := objects[row.DeliveredThumbnail.Key].body
	require.Greater(t, len(full), 12)
	require.Equal(t, "ftyp", string(full[4:8]))
	require.Greater(t, len(thumb), 12)
	require.Equal(t, "WEBP", string(thumb[8:12]))
	fullHash := sha256.Sum256(full)
	require.Equal(t, fullHash[:], snapshot.DeliveredSha256)
	require.EqualValues(t, len(full), snapshot.DeliveredSizeBytes)
	require.Equal(t, "video/mp4", snapshot.DeliveredMimeType)
	t.Logf("actual source=%dB mp4=%dB webp=%dB revision=%d file=%s", len(body), len(full), len(thumb), snapshot.ObjectRevision, imported.FileId)
}
