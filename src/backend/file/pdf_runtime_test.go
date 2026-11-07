package main

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
	messagingv1 "voice.app/voice/messaging/v1"
	"voice/backend/file/internal/store"
)

func pdfRuntimeFixture() []byte {
	var b bytes.Buffer
	b.WriteString("%PDF-1.4\n")
	blue, red := "0 0 1 rg 0 0 200 100 re f\n", "1 0 0 rg 0 0 100 200 re f\n"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 100] /Resources << >> /Contents 5 0 R >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 200] /Resources << >> /Contents 6 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(blue), blue),
		fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(red), red),
	}
	var offsets []int
	for i, obj := range objects {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, offset := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(objects)+1, xref)
	return b.Bytes()
}

type pdfRuntimeChat struct {
	chatv1.UnimplementedChatServiceServer
	profile, chat string
	created       time.Time
}

func (p *pdfRuntimeChat) CheckMessageReadEntitlement(_ context.Context, r *chatv1.CheckMessageReadEntitlementRequest) (*chatv1.CheckMessageReadEntitlementResponse, error) {
	return &chatv1.CheckMessageReadEntitlementResponse{Entitled: r.GetProfileId() == p.profile && r.GetChatId() == p.chat && r.GetMessageCreatedAt().AsTime().Equal(p.created)}, nil
}

type pdfRuntimeMessaging struct {
	messagingv1.UnimplementedMessagingServiceServer
	message, chat string
	created       time.Time
}

func (p *pdfRuntimeMessaging) GetMessage(_ context.Context, r *messagingv1.GetMessageRequest) (*messagingv1.GetMessageResponse, error) {
	if r.GetMessageId() != p.message {
		return nil, status.Error(codes.NotFound, "fixture message not found")
	}
	return &messagingv1.GetMessageResponse{Message: &messagingv1.Message{Id: p.message, Chat: &chatv1.ChatRef{Id: p.chat}, CreatedAt: timestamppb.New(p.created)}}, nil
}

func pdfRuntimeGRPC(t *testing.T, register func(*grpc.Server)) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	register(server)
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Stop(); _ = listener.Close() })
	return listener.Addr().String()
}
func pdfRuntimeAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	return address
}

// Run the actual production main in a child with an allowlisted environment;
// no inherited service endpoint, key, or shared deployment can be selected.
func TestPDFRuntimeMainProcess(t *testing.T) {
	if os.Getenv("VOICE_PDF_MAIN_PROCESS") == "1" {
		main()
	}
}

func TestPDFRuntimeMainUploadsRendersAndRevokesThumbnail(t *testing.T) {
	if testing.Short() {
		t.Skip("owned PostgreSQL/runtime integration")
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("Poppler unavailable on host; image proof is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool := startPDFPostgres(t, ctx)
	defer pool.Close()
	var mutex sync.Mutex
	type object struct {
		body []byte
		mime string
	}
	objects := map[string]object{}
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, "/pdf-runtime/")
		switch r.Method {
		case http.MethodPut:
			body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
			if err != nil {
				w.WriteHeader(500)
				return
			}
			mutex.Lock()
			objects[key] = object{body: body, mime: r.Header.Get("Content-Type")}
			mutex.Unlock()
			w.WriteHeader(http.StatusOK)
		case http.MethodGet:
			mutex.Lock()
			item, ok := objects[key]
			mutex.Unlock()
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", item.mime)
			_, _ = w.Write(item.body)
		case http.MethodDelete:
			mutex.Lock()
			delete(objects, key)
			mutex.Unlock()
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	defer storage.Close()
	profile, message, chat := uuid.NewString(), uuid.NewString(), uuid.NewString()
	created := time.Now().UTC().Truncate(time.Second)
	chatAddress := pdfRuntimeGRPC(t, func(s *grpc.Server) {
		chatv1.RegisterChatServiceServer(s, &pdfRuntimeChat{profile: profile, chat: chat, created: created})
	})
	messagingAddress := pdfRuntimeGRPC(t, func(s *grpc.Server) {
		messagingv1.RegisterMessagingServiceServer(s, &pdfRuntimeMessaging{message: message, chat: chat, created: created})
	})
	grpcAddress, httpAddress := pdfRuntimeAddress(t), pdfRuntimeAddress(t)
	childCtx, stopChild := context.WithCancel(ctx)
	command := exec.CommandContext(childCtx, os.Args[0], "-test.run=^TestPDFRuntimeMainProcess$")
	for _, key := range []string{"PATH", "SYSTEMROOT", "WINDIR", "TEMP", "TMP", "HOME", "USERPROFILE"} {
		if value, ok := os.LookupEnv(key); ok {
			command.Env = append(command.Env, key+"="+value)
		}
	}
	// pgx ConnString returns the original DSN, not runtime parameters added by
	// the private-schema fixture. Preserve that schema for the child process.
	databaseURL := pool.Config().ConnString()
	if schema := pool.Config().ConnConfig.RuntimeParams["search_path"]; schema != "" {
		parsed, parseErr := url.Parse(databaseURL)
		require.NoError(t, parseErr)
		require.Contains(t, []string{"postgres", "postgresql"}, parsed.Scheme)
		query := parsed.Query()
		query.Set("search_path", schema)
		parsed.RawQuery = query.Encode()
		databaseURL = parsed.String()
	}
	command.Env = append(command.Env, "VOICE_PDF_MAIN_PROCESS=1", "DATABASE_URL="+databaseURL, "FILE_GRPC_LISTEN="+grpcAddress, "LISTEN_ADDR="+httpAddress,
		"FILE_R2_ENDPOINT="+storage.URL, "FILE_R2_REGION=auto", "FILE_R2_BUCKET=pdf-runtime", "FILE_R2_ACCESS_KEY_ID=test-access", "FILE_R2_SECRET_ACCESS_KEY=test-secret",
		"CHAT_GRPC_ADDR="+chatAddress, "MESSAGING_GRPC_ADDR="+messagingAddress, "FILE_DOWNLOAD_SIGNING_KEY=0123456789abcdef0123456789abcdef", "FILE_DOWNLOAD_BASE_URL=http://"+httpAddress+"/api/v1/files/download")
	logPath := filepath.Join(t.TempDir(), "file-main.log")
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE, 0600)
	require.NoError(t, err)
	command.Stdout, command.Stderr = logFile, logFile
	require.NoError(t, command.Start())
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	defer func() {
		stopChild()
		<-done
		_ = logFile.Close()
		if t.Failed() {
			raw, _ := os.ReadFile(logPath)
			lines := strings.Split(string(raw), "\n")
			if len(lines) > 12 {
				lines = append(append(lines[:6], "..."), lines[len(lines)-6:]...)
			}
			t.Log(strings.Join(lines, "\n"))
		}
	}()
	connection, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer connection.Close()
	readyCtx, stopReady := context.WithTimeout(ctx, 10*time.Second)
	defer stopReady()
	require.NoError(t, waitForGRPCReady(readyCtx, connection))
	client := filev1.NewFileServiceClient(connection)
	user := metadata.NewOutgoingContext(ctx, metadata.Pairs("x-voice-profile-id", profile))
	raw := pdfRuntimeFixture()
	upload, err := client.RequestUpload(user, &filev1.RequestUploadRequest{OriginalName: "runtime.pdf", MimeType: "application/pdf", SizeBytes: int64(len(raw))})
	require.NoError(t, err)
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, upload.UploadResponse.PresignedPutUrl, bytes.NewReader(raw))
	require.NoError(t, err)
	request.Header.Set("Content-Type", "application/pdf")
	response, err := http.DefaultClient.Do(request)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	_ = response.Body.Close()
	id := upload.UploadResponse.FileId
	confirmed, err := client.ConfirmUpload(user, &filev1.ConfirmUploadRequest{FileId: id, Sha256Hash: sha256Hex(raw)})
	require.NoError(t, err)
	require.Equal(t, "processing", confirmed.FileMetadata.Status)
	var ready *filev1.FileMetadata
	require.Eventually(t, func() bool {
		r, e := client.GetFileMetadata(user, &filev1.GetFileMetadataRequest{FileId: id})
		if e != nil {
			return false
		}
		ready = r.FileMetadata
		return ready.Status == "ready"
	}, 8*time.Second, 100*time.Millisecond, "production main must register the PDF worker")
	require.Equal(t, "application/pdf", ready.MimeType)
	require.EqualValues(t, len(raw), ready.SizeBytes)
	require.Equal(t, sha256Hex(raw), ready.Sha256Hash)
	require.NotNil(t, ready.ThumbnailR2Key)
	_, err = pool.Exec(ctx, `INSERT INTO file_references(file_id,owner_type,owner_id) VALUES($1,1,$2)`, id, message)
	require.NoError(t, err)
	access := &filev1.FileAccessSelector{Selector: &filev1.FileAccessSelector_Reference{Reference: &filev1.FileReferenceKey{FileId: id, OwnerType: filev1.FileReferenceOwnerType_FILE_REFERENCE_OWNER_TYPE_MESSAGE, OwnerId: message}}}
	thumbnail, err := client.GetFileURL(user, &filev1.GetFileURLRequest{FileId: id, Access: access, Variant: filev1.FileURLVariant_FILE_URL_VARIANT_THUMBNAIL})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(thumbnail.PresignedGetUrl, "http://"+httpAddress+"/api/v1/files/download/"))
	response, err = http.Get(thumbnail.PresignedGetUrl)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "image/png", response.Header.Get("Content-Type"))
	image, err := png.Decode(response.Body)
	_ = response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, 320, image.Bounds().Dx())
	require.Equal(t, 160, image.Bounds().Dy())
	r, g, b, _ := image.At(160, 80).RGBA()
	require.Greater(t, b, r)
	require.Greater(t, b, g)
	original, err := client.GetFileURL(user, &filev1.GetFileURLRequest{FileId: id, Access: access})
	require.NoError(t, err)
	response, err = http.Get(original.PresignedGetUrl)
	require.NoError(t, err)
	require.Equal(t, "application/pdf", response.Header.Get("Content-Type"))
	downloaded, err := io.ReadAll(response.Body)
	_ = response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, raw, downloaded)
	_, err = pool.Exec(ctx, `UPDATE file_references SET released_at=now() WHERE file_id=$1`, id)
	require.NoError(t, err)
	response, err = http.Get(thumbnail.PresignedGetUrl)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	_ = response.Body.Close()
}

func TestPDFRuntimeRequiresInstalledDependencies(t *testing.T) {
	require.ErrorContains(t, startPDFRuntime(context.Background(), nil, nil, nil), "persistence and object storage")
	if testing.Short() {
		t.Skip("owned PostgreSQL startup readiness")
	}
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("Poppler unavailable on host")
	}
	ctx, cancel := context.WithCancel(context.Background())
	pool := startFilePostgres(t, ctx)
	defer pool.Close()
	defer cancel()
	files := store.NewFilesStore(pool)
	require.ErrorContains(t, startPDFRuntime(ctx, files, &pdfStorage{}, nil), "migrations 000010 and 000011")
	for _, name := range []string{"000009_gif_processing", "000010_catalog_asset_import", "000011_tracked_write_owner_kind"} {
		applyFileSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", name+".up.sql"))
	}
	_, err := pool.Exec(ctx, `ALTER TABLE file_blobs DISABLE TRIGGER file_catalog_guard_blob_gc`)
	require.NoError(t, err)
	require.ErrorContains(t, startPDFRuntime(ctx, files, &pdfStorage{}, nil), "active tracked-write GC guard")
	_, err = pool.Exec(ctx, `ALTER TABLE file_blobs ENABLE TRIGGER file_catalog_guard_blob_gc`)
	require.NoError(t, err)
	t.Setenv("PATH", t.TempDir())
	require.ErrorContains(t, startPDFRuntime(ctx, files, &pdfStorage{}, nil), "renderer unavailable")
}
