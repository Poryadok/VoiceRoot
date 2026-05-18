package grpcsvc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

func init() {
	if runtime.GOOS == "windows" && os.Getenv("TESTCONTAINERS_RYUK_DISABLED") == "" {
		_ = os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startChatPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("chatdb"),
		postgres.WithUsername("u"),
		postgres.WithPassword("p"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	connStr = strings.Replace(connStr, "localhost", "127.0.0.1", 1)
	connStr = strings.Replace(connStr, "[::1]", "127.0.0.1", 1)

	var pool *pgxpool.Pool
	for i := 0; i < 60; i++ {
		p, err := pgxpool.New(ctx, connStr)
		if err == nil {
			if pingErr := p.Ping(ctx); pingErr == nil {
				pool = p
				break
			}
			p.Close()
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.NotNil(t, pool, "postgres did not become ready in time")
	t.Cleanup(pool.Close)
	return pool
}

func applyChatMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}

func withAccountProfileCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

type mapProfileAccounts map[uuid.UUID]uuid.UUID

func (m mapProfileAccounts) AccountIDByProfileID(_ context.Context, profileID uuid.UUID) (uuid.UUID, error) {
	a, ok := m[profileID]
	if !ok {
		return uuid.Nil, status.Error(codes.NotFound, "profile not found")
	}
	return a, nil
}

type stubBlocks struct {
	blocked bool
	err     error
}

func (s stubBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return s.blocked, s.err
}

type chatServerOption func(*ChatGRPC)

// WithChatEventsPublisher wires optional NATS chat.events publisher for integration tests.
func WithChatEventsPublisher(p chatevents.Publisher) chatServerOption {
	return func(c *ChatGRPC) { c.ChatEvents = p }
}

func startChatGRPCTestServer(t *testing.T, pool *pgxpool.Pool, profiles UserProfileLookup, blocks AccountBlockChecker, enrich ListChatsEnrichment, opts ...chatServerOption) (chatv1.ChatServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	svc := &ChatGRPC{
		DM:         &store.DMStore{Pool: pool},
		Profiles:   profiles,
		Blocks:     blocks,
		ListEnrich: enrich,
	}
	for _, o := range opts {
		o(svc)
	}
	chatv1.RegisterChatServiceServer(srv, svc)
	go func() {
		if err := srv.Serve(lis); err != nil {
			t.Logf("grpc serve: %v", err)
		}
	}()
	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	cleanup := func() {
		_ = conn.Close()
		srv.Stop()
	}
	return chatv1.NewChatServiceClient(conn), cleanup
}

// TestCreateDM_GetDM_NoFriendshipRequired documents PLAN Phase 1: DM without friendship; only blocks gate.
func TestCreateDM_GetDM_NoFriendshipRequired(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	r1, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.NotNil(t, r1.GetChat())
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, r1.GetChat().GetType())
	require.Equal(t, profA.String(), r1.GetChat().GetCreatorProfileId())

	r2, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.Equal(t, r1.GetChat().GetId(), r2.GetChat().GetId(), "GetDM must return existing DM")

	ctxB := withAccountProfileCtx(ctx, accB, profB)
	r3, err := client.CreateDM(ctxB, &chatv1.CreateDMRequest{OtherProfileId: profA.String()})
	require.NoError(t, err)
	require.Equal(t, r1.GetChat().GetId(), r3.GetChat().GetId(), "other participant must resolve to same chat")
}

func TestCreateDM_BlockedPair_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, stubBlocks{blocked: true}, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestGetDM_BlockedPair_PermissionDenied documents chat-service.md: Social blocks gate DM; GetDM uses the same path as CreateDM.
func TestGetDM_BlockedPair_PermissionDenied(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, stubBlocks{blocked: true}, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))
}

// TestGetDM_Idempotent_RepeatedCallsSameChat documents stable DM identity: find-or-create must not fork rows.
func TestGetDM_Idempotent_RepeatedCallsSameChat(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	var firstID string
	for i := 0; i < 5; i++ {
		r, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
		require.NoError(t, err)
		id := r.GetChat().GetId()
		if firstID == "" {
			firstID = id
		} else {
			require.Equal(t, firstID, id, "GetDM must be idempotent")
		}
	}
	require.NotEmpty(t, firstID)
}

// TestGetDM_Stranger_OpensDialogWithoutCreateDM documents PLAN Phase 1: no friendship required; either side may resolve the DM via GetDM alone.
func TestGetDM_Stranger_OpensDialogWithoutCreateDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	accB := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA, profB: accB}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	rA, err := client.GetDM(ctxA, &chatv1.GetDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	require.Equal(t, chatv1.ChatType_CHAT_TYPE_DM, rA.GetChat().GetType())
	require.Equal(t, profA.String(), rA.GetChat().GetCreatorProfileId())

	ctxB := withAccountProfileCtx(ctx, accB, profB)
	rB, err := client.GetDM(ctxB, &chatv1.GetDMRequest{OtherProfileId: profA.String()})
	require.NoError(t, err)
	require.Equal(t, rA.GetChat().GetId(), rB.GetChat().GetId(), "stranger B must join same DM without prior CreateDM")
}

func TestCreateDM_Self_InvalidArgument(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	acc := uuid.New()
	prof := uuid.New()
	profiles := mapProfileAccounts{prof: acc}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, acc, prof)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: prof.String()})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestCreateDM_UnknownProfile_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	accA := uuid.New()
	profA := uuid.New()
	profB := uuid.New()
	profiles := mapProfileAccounts{profA: accA}

	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	ctxA := withAccountProfileCtx(ctx, accA, profA)
	_, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestCreateDM_MissingAuth_Unauthenticated(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startChatPostgresForTest(t, ctx)
	applyChatMigration(t, ctx, pool)

	profB := uuid.New()
	profiles := mapProfileAccounts{profB: uuid.New()}
	client, cleanup := startChatGRPCTestServer(t, pool, profiles, nil, nil)
	t.Cleanup(cleanup)

	_, err := client.CreateDM(ctx, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))
}
