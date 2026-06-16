package grpcsvc

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/file/internal/store"

	filev1 "voice.app/voice/file/v1"
)

type recordingScanner struct {
	calls int
}

func (s *recordingScanner) ScanBytes(context.Context, []byte) (string, error) {
	s.calls++
	return "clean", nil
}

type gateObjectReader map[string][]byte

func (r gateObjectReader) ReadObject(_ context.Context, key string, _ int64) ([]byte, error) {
	if b, ok := r[key]; ok {
		return b, nil
	}
	return nil, errors.New("object not found")
}

// TestConfirmUpload_E2E_SkipsClamAVScan documents E2E ciphertext blobs skip malware scan (docs/features/encryption.md).
func TestConfirmUpload_E2E_SkipsClamAVScan(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startFileGatePostgres(t, ctx)
	chatID := uuid.New()
	prof := uuid.New()
	acct := uuid.New()
	guard := e2eGateGuard{
		members:    map[uuid.UUID][]uuid.UUID{chatID: {prof}},
		e2eEnabled: map[uuid.UUID]bool{chatID: true},
		chatTypes:  map[uuid.UUID]string{chatID: "dm"},
	}
	scanner := &recordingScanner{}
	presigner := gatePresigner{}
	client := startFileGateGRPCWithScan(t, pool, guard, presigner, scanner, gateObjectReader{})

	isE2E := true
	uploadResp, err := client.RequestUpload(fileGateCtx(ctx, acct, prof), &filev1.RequestUploadRequest{
		OriginalName: "cipher.zip",
		MimeType:     "application/zip",
		SizeBytes:    128,
		ContextChat:  chatDMRefGate(chatID),
		IsE2E:        &isE2E,
	})
	require.NoError(t, err)
	r2Key := uploadResp.GetUploadResponse().GetR2Key()
	client = startFileGateGRPCWithScan(t, pool, guard, presigner, scanner, gateObjectReader{
		r2Key: []byte("PK\x03\x04encrypted"),
	})

	confirmed, err := client.ConfirmUpload(fileGateCtx(ctx, acct, prof), &filev1.ConfirmUploadRequest{
		FileId:     uploadResp.GetUploadResponse().GetFileId(),
		Sha256Hash: "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
	})
	require.NoError(t, err)
	meta := confirmed.GetFileMetadata()
	require.Equal(t, "ready", meta.GetStatus())
	require.True(t, meta.GetIsE2E())
	require.Equal(t, "skipped", meta.GetScanResult())
	require.Equal(t, 0, scanner.calls, "E2E confirm must not invoke ClamAV scanner")
}

func startFileGateGRPCWithScan(
	t *testing.T,
	pool *pgxpool.Pool,
	guard e2eGateGuard,
	presigner gatePresigner,
	scanner Scanner,
	reader ObjectReader,
) filev1.FileServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	filev1.RegisterFileServiceServer(srv, New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: presigner,
		ChatGuard: guard,
		Reader:    reader,
		Scanner:   scanner,
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
