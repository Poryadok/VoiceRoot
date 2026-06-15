package grpcsvc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
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

	"voice/backend/file/internal/authctx"
	"voice/backend/file/internal/r2file"
	"voice/backend/file/internal/store"
	"voice/backend/pkg/integrationtest"

	chatv1 "voice.app/voice/chat/v1"
	filev1 "voice.app/voice/file/v1"
)

// Batch E2E-A audit: File service must gate is_e2e uploads by chat.e2e_enabled (docs/TODO.md).

type e2eGateGuard struct {
	members    map[uuid.UUID][]uuid.UUID
	e2eEnabled map[uuid.UUID]bool
	chatTypes  map[uuid.UUID]string
}

func (g e2eGateGuard) EnsureMember(_ context.Context, chatID, profileID uuid.UUID) error {
	for _, member := range g.members[chatID] {
		if member == profileID {
			return nil
		}
	}
	return ErrNotChatMember
}

func (g e2eGateGuard) ChatE2EState(_ context.Context, chatID uuid.UUID) (chatType string, e2eEnabled bool, err error) {
	typ, ok := g.chatTypes[chatID]
	if !ok {
		return "", false, ErrNotChatMember
	}
	return typ, g.e2eEnabled[chatID], nil
}

type gatePresigner struct{}

func (gatePresigner) PresignPut(_ context.Context, in r2file.PutPresignInput) (string, error) {
	return "https://r2.example/upload/" + in.Key, nil
}

func (gatePresigner) PresignGet(_ context.Context, in r2file.GetPresignInput) (string, error) {
	return "https://r2.example/download/" + in.Key, nil
}

func TestRequestUpload_E2E_RejectedWhenChatNotE2EEnabled(t *testing.T) {
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
		e2eEnabled: map[uuid.UUID]bool{chatID: false},
		chatTypes:  map[uuid.UUID]string{chatID: "dm"},
	}
	client := startFileGateGRPC(t, pool, guard)

	isE2E := true
	_, err := client.RequestUpload(fileGateCtx(ctx, acct, prof), &filev1.RequestUploadRequest{
		OriginalName: "secret.bin",
		MimeType:     "application/octet-stream",
		SizeBytes:    128,
		ContextChat:  chatDMRefGate(chatID),
		IsE2E:        &isE2E,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRequestUpload_NonE2E_RejectedWhenChatE2EEnabled(t *testing.T) {
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
	client := startFileGateGRPC(t, pool, guard)

	isE2E := false
	_, err := client.RequestUpload(fileGateCtx(ctx, acct, prof), &filev1.RequestUploadRequest{
		OriginalName: "plain.txt",
		MimeType:     "text/plain",
		SizeBytes:    64,
		ContextChat:  chatDMRefGate(chatID),
		IsE2E:        &isE2E,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestRequestUpload_E2E_RejectedForGroupChat(t *testing.T) {
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
		chatTypes:  map[uuid.UUID]string{chatID: "group"},
	}
	client := startFileGateGRPC(t, pool, guard)

	isE2E := true
	_, err := client.RequestUpload(fileGateCtx(ctx, acct, prof), &filev1.RequestUploadRequest{
		OriginalName: "group-secret.bin",
		MimeType:     "application/octet-stream",
		SizeBytes:    256,
		ContextChat:  chatGroupRefGate(chatID),
		IsE2E:        &isE2E,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func chatDMRefGate(chatID uuid.UUID) *chatv1.ChatRef {
	dm := chatv1.ChatType_CHAT_TYPE_DM
	return &chatv1.ChatRef{Id: chatID.String(), Type: &dm}
}

func chatGroupRefGate(chatID uuid.UUID) *chatv1.ChatRef {
	group := chatv1.ChatType_CHAT_TYPE_GROUP
	return &chatv1.ChatRef{Id: chatID.String(), Type: &group}
}

func fileGateCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

func startFileGateGRPC(t *testing.T, pool *pgxpool.Pool, guard e2eGateGuard) filev1.FileServiceClient {
	t.Helper()
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer()
	filev1.RegisterFileServiceServer(srv, New(Deps{
		Files:     store.NewFilesStore(pool),
		Presigner: gatePresigner{},
		ChatGuard: guard,
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

func startFileGatePostgres(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "file_e2e_gate", "")
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000001_init.up.sql"))
	applyFileGateSQL(t, ctx, pool, filepath.Join("src", "backend", "migrations", "file_db", "000002_premium_upload_limit.up.sql"))
	return pool
}

func applyFileGateSQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, relPath string) {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(fileGateRepoRoot(t), relPath))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(b))
	require.NoError(t, err)
}

func fileGateRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}
