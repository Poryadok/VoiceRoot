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
	"google.golang.org/protobuf/proto"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// .../internal/grpcsvc/user_integration_test.go -> repo root is 5 parents up
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestProfileGRPC_v1DDL(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()

	pgC, err := postgres.Run(ctx, "postgres:16-bookworm",
		postgres.BasicWaitStrategies(),
		postgres.WithDatabase("userdb"),
		postgres.WithUsername("u"),
		postgres.WithPassword("p"),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	if strings.Contains(connStr, "localhost") {
		connStr = strings.Replace(connStr, "localhost", "127.0.0.1", 1)
	}
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

	migrationPath := filepath.Join(repoRoot(t), "src", "backend", "migrations", "user_db", "000001_init.up.sql")
	sqlBytes, err := os.ReadFile(migrationPath)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)

	accountA := uuid.New()
	accountB := uuid.New()
	pid := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'alice', '0001', 'Alice', true)`,
		pid, accountA)
	require.NoError(t, err)

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, &UserGRPC{Profiles: store.NewProfileStore(pool)})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	cli := userv1.NewUserServiceClient(conn)

	t.Run("GetProfile by id", func(t *testing.T) {
		resp, err := cli.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_ProfileId{ProfileId: pid.String()},
		})
		require.NoError(t, err)
		require.Equal(t, pid.String(), resp.GetProfile().GetId())
		require.Equal(t, "alice", resp.GetProfile().GetUsername())
		require.Equal(t, "0001", resp.GetProfile().GetDiscriminator())
		require.True(t, resp.GetProfile().GetIsPrimary())
	})

	t.Run("GetProfile by handle", func(t *testing.T) {
		resp, err := cli.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_Username{Username: "alice#0001"},
		})
		require.NoError(t, err)
		require.Equal(t, pid.String(), resp.GetProfile().GetId())
	})

	t.Run("GetProfile invalid handle", func(t *testing.T) {
		_, err := cli.GetProfile(ctx, &userv1.GetProfileRequest{
			By: &userv1.GetProfileRequest_Username{Username: "alice"},
		})
		require.Error(t, err)
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("GetProfiles batch", func(t *testing.T) {
		pid2 := uuid.New()
		_, err := pool.Exec(ctx, `
			INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
			VALUES ($1, $2, 'bob', '0002', 'Bob', false)`,
			pid2, accountA)
		require.NoError(t, err)

		resp, err := cli.GetProfiles(ctx, &userv1.GetProfilesRequest{
			ProfileIds: []string{pid.String(), pid2.String()},
		})
		require.NoError(t, err)
		require.Len(t, resp.GetProfileList().GetProfiles(), 2)
	})

	t.Run("UpdateProfile forbidden for other account", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountB.String())
		_, err := cli.UpdateProfile(mdCtx, &userv1.UpdateProfileRequest{
			ProfileId:   pid.String(),
			DisplayName: proto.String("Hacker"),
		})
		require.Error(t, err)
		require.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("UpdateProfile ok", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.UpdateProfile(mdCtx, &userv1.UpdateProfileRequest{
			ProfileId:   pid.String(),
			DisplayName: proto.String("Alice II"),
			Locale:      proto.String("en"),
			Theme:       proto.String("light"),
		})
		require.NoError(t, err)
		require.Equal(t, "Alice II", resp.GetProfile().GetDisplayName())
		require.Equal(t, "en", resp.GetProfile().GetLocale())
		require.Equal(t, "light", resp.GetProfile().GetTheme())
	})

	t.Run("CreateProfile secondary", func(t *testing.T) {
		mdCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountA.String())
		resp, err := cli.CreateProfile(mdCtx, &userv1.CreateProfileRequest{
			DisplayName: "Alt",
			Username:    proto.String("altuser"),
		})
		require.NoError(t, err)
		require.False(t, resp.GetProfile().GetIsPrimary())
		require.Equal(t, accountA.String(), resp.GetProfile().GetAccountId())
		require.Equal(t, "Alt", resp.GetProfile().GetDisplayName())
		require.NotEmpty(t, resp.GetProfile().GetDiscriminator())
	})
}
