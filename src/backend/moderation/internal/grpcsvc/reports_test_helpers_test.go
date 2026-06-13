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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/moderation/internal/store"

	moderationv1 "voice.app/voice/moderation/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func integrationtestApplySQL(t *testing.T, ctx context.Context, pool *pgxpool.Pool, path string) {
	t.Helper()
	sqlBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sqlBytes))
	require.NoError(t, err)
}

func startModerationGRPCTestServer(t *testing.T, pool *pgxpool.Pool) (moderationv1.ModerationServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	moderationv1.RegisterModerationServiceServer(srv, &ModerationGRPC{
		Reports: &store.ReportStore{Pool: pool},
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
	return moderationv1.NewModerationServiceClient(conn), cleanup
}

func withReporterProfile(ctx context.Context, profileID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, "x-voice-profile-id", profileID.String())
}
