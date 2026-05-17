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

	"voice/backend/social/internal/authctx"
	"voice/backend/social/internal/store"

	commonv1 "voice.app/voice/common/v1"
	socialv1 "voice.app/voice/social/v1"
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

func startSocialGRPCTestServer(t *testing.T, pool *pgxpool.Pool) (socialv1.SocialServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, &SocialGRPC{
		Friends: &store.FriendshipStore{Pool: pool},
		Blocks:  &store.BlockStore{Pool: pool},
	})
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
	return socialv1.NewSocialServiceClient(conn), cleanup
}

func withProfileCtx(ctx context.Context, profileID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

func TestFriendFlow_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("socialdb"),
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

	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "social_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	a := uuid.New()
	b := uuid.New()

	// Missing credentials
	_, err = client.SendFriendInvitation(ctx, &socialv1.SendFriendInvitationRequest{TargetProfileId: b.String()})
	require.Error(t, err)
	require.Equal(t, codes.Unauthenticated, status.Code(err))

	// Self invite
	_, err = client.SendFriendInvitation(withProfileCtx(ctx, a), &socialv1.SendFriendInvitationRequest{TargetProfileId: a.String()})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))

	// A invites B
	_, err = client.SendFriendInvitation(withProfileCtx(ctx, a), &socialv1.SendFriendInvitationRequest{TargetProfileId: b.String()})
	require.NoError(t, err)

	// Idempotent pending
	_, err = client.SendFriendInvitation(withProfileCtx(ctx, a), &socialv1.SendFriendInvitationRequest{TargetProfileId: b.String()})
	require.NoError(t, err)

	// B cannot send invite to A while incoming pending exists
	_, err = client.SendFriendInvitation(withProfileCtx(ctx, b), &socialv1.SendFriendInvitationRequest{TargetProfileId: a.String()})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))

	// List requests
	lr, err := client.ListFriendRequests(withProfileCtx(ctx, b), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, lr.GetFriendRequestList().GetIncoming(), 1)
	require.Equal(t, a.String(), lr.GetFriendRequestList().GetIncoming()[0].GetProfileId())
	lo, err := client.ListFriendRequests(withProfileCtx(ctx, a), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, lo.GetFriendRequestList().GetOutgoing(), 1)
	require.Equal(t, b.String(), lo.GetFriendRequestList().GetOutgoing()[0].GetProfileId())

	// Accept
	_, err = client.AcceptFriendInvitation(withProfileCtx(ctx, b), &socialv1.AcceptFriendInvitationRequest{RequesterProfileId: a.String()})
	require.NoError(t, err)

	// Already friends — second invite
	_, err = client.SendFriendInvitation(withProfileCtx(ctx, a), &socialv1.SendFriendInvitationRequest{TargetProfileId: b.String()})
	require.Error(t, err)
	require.Equal(t, codes.AlreadyExists, status.Code(err))

	lf, err := client.ListFriends(withProfileCtx(ctx, a), &socialv1.ListFriendsRequest{
		Page: &commonv1.CursorPageRequest{PageSize: 10},
	})
	require.NoError(t, err)
	require.Len(t, lf.GetFriendList().GetFriends(), 1)
	require.Equal(t, b.String(), lf.GetFriendList().GetFriends()[0].GetProfileId())

	// Remove
	_, err = client.RemoveFriend(withProfileCtx(ctx, a), &socialv1.RemoveFriendRequest{FriendProfileId: b.String()})
	require.NoError(t, err)
	lf2, err := client.ListFriends(withProfileCtx(ctx, a), &socialv1.ListFriendsRequest{})
	require.NoError(t, err)
	require.Empty(t, lf2.GetFriendList().GetFriends())
}

func TestFriendDecline_OutgoingStillVisible(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("socialdb"),
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
	require.NotNil(t, pool)
	t.Cleanup(pool.Close)

	sqlBytes, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "social_db", "000001_init.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	a := uuid.New()
	b := uuid.New()
	_, err = client.SendFriendInvitation(withProfileCtx(ctx, a), &socialv1.SendFriendInvitationRequest{TargetProfileId: b.String()})
	require.NoError(t, err)
	_, err = client.DeclineFriendInvitation(withProfileCtx(ctx, b), &socialv1.DeclineFriendInvitationRequest{RequesterProfileId: a.String()})
	require.NoError(t, err)

	out, err := client.ListFriendRequests(withProfileCtx(ctx, a), &socialv1.ListFriendRequestsRequest{})
	require.NoError(t, err)
	require.Len(t, out.GetFriendRequestList().GetOutgoing(), 1)
	require.Equal(t, b.String(), out.GetFriendRequestList().GetOutgoing()[0].GetProfileId())
}

func TestFriendListFriends_InvalidCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("socialdb"),
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
	require.NotNil(t, pool)
	t.Cleanup(pool.Close)

	sqlBytes, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "social_db", "000001_init.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	client, cleanup := startSocialGRPCTestServer(t, pool)
	t.Cleanup(cleanup)

	a := uuid.New()
	_, err = client.ListFriends(withProfileCtx(ctx, a), &socialv1.ListFriendsRequest{
		Page: &commonv1.CursorPageRequest{Cursor: "garbage", PageSize: 5},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}
