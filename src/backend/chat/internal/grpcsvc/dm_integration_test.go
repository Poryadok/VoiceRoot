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
	"voice/backend/pkg/integrationtest"

	"voice/backend/chat/internal/authctx"
	"voice/backend/chat/internal/chatevents"
	"voice/backend/chat/internal/store"

	chatv1 "voice.app/voice/chat/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func startChatPostgresForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, "chatdb", "")
}

func applyChatMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, name := range []string{"000001_init.up.sql", "000002_dm_requests.up.sql", "000003_groups.up.sql", "000004_slow_mode.up.sql", "000005_thread_settings.up.sql"} {
		migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "chat_db", name)
		sqlBytes, err := os.ReadFile(migrationPath)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
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

func TestDMRequestsInboxAcceptDecline(t *testing.T) {
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
	ctxB := withAccountProfileCtx(ctx, accB, profB)
	dm, err := client.CreateDM(ctxA, &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	chatID := dm.GetChat().GetId()

	mainList, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Empty(t, mainList.GetChatList().GetItems())

	requests := "requests"
	requestList, err := client.ListChats(ctxB, &chatv1.ListChatsRequest{Inbox: &requests})
	require.NoError(t, err)
	require.Len(t, requestList.GetChatList().GetItems(), 1)
	require.Equal(t, chatID, requestList.GetChatList().GetItems()[0].GetChat().GetId())
	require.True(t, requestList.GetChatList().GetItems()[0].GetIsStranger())

	_, err = client.AcceptDMRequest(ctxB, &chatv1.AcceptDMRequestRequest{ChatId: chatID})
	require.NoError(t, err)
	mainList, err = client.ListChats(ctxB, &chatv1.ListChatsRequest{})
	require.NoError(t, err)
	require.Len(t, mainList.GetChatList().GetItems(), 1)

	profC := uuid.New()
	accC := uuid.New()
	profiles[profC] = accC
	dm2, err := client.CreateDM(withAccountProfileCtx(ctx, accC, profC), &chatv1.CreateDMRequest{OtherProfileId: profB.String()})
	require.NoError(t, err)
	_, err = client.DeclineDMRequest(ctxB, &chatv1.DeclineDMRequestRequest{ChatId: dm2.GetChat().GetId()})
	require.NoError(t, err)
	requestList, err = client.ListChats(ctxB, &chatv1.ListChatsRequest{Inbox: &requests})
	require.NoError(t, err)
	for _, item := range requestList.GetChatList().GetItems() {
		require.NotEqual(t, dm2.GetChat().GetId(), item.GetChat().GetId())
	}
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
