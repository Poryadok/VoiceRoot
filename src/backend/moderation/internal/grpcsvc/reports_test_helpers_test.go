package grpcsvc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/moderation/internal/intakeprincipal"
	"voice/backend/moderation/internal/reportintake"
	"voice/backend/moderation/internal/store"
	"voice/backend/pkg/principal"

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

func startModerationPostgresPlatform(t *testing.T, _ context.Context) *pgxpool.Pool {
	t.Helper()
	return reportIntakePool(t)
}

func startModerationGRPCTestServer(t *testing.T, pool *pgxpool.Pool, opts ...func(*ModerationGRPC)) (moderationv1.ModerationServiceClient, func()) {
	t.Helper()
	const bufSize = 1 << 20
	lis := bufconn.Listen(bufSize)
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "test", PrivateKey: key})
	require.NoError(t, err)
	verifier := intakeprincipal.Verifier{Keys: func(context.Context, string, string) (*rsa.PublicKey, error) { return &key.PublicKey, nil }, Replay: func(context.Context, string, string, time.Time) error { return nil }, Epoch: func(context.Context, string, int64) error { return nil }}
	srv := grpc.NewServer(grpc.UnaryInterceptor(func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if info.FullMethod == moderationv1.ModerationService_CreateReport_FullMethodName {
			return verifier.Unary()(ctx, request, info, handler)
		}
		return handler(ctx, request)
	}))
	svc := &ModerationGRPC{
		Reports:   &store.ReportStore{Pool: pool},
		Sanctions: &store.SanctionStore{Pool: pool},
		Appeals:   &store.AppealStore{Pool: pool},
		AuditLog:  &store.AuditLogStore{Pool: pool},
		AutoMod:   &store.AutoModStore{Pool: pool},
	}
	for _, o := range opts {
		o(svc)
	}
	moderationv1.RegisterModerationServiceServer(srv, svc)
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
		grpc.WithUnaryInterceptor(func(ctx context.Context, method string, request, reply any, conn *grpc.ClientConn, invoke grpc.UnaryInvoker, options ...grpc.CallOption) error {
			if method == moderationv1.ModerationService_CreateReport_FullMethodName {
				md, _ := metadata.FromOutgoingContext(ctx)
				profiles := md.Get("x-voice-profile-id")
				if len(profiles) == 1 {
					req := request.(*moderationv1.CreateReportRequest)
					hash, err := principal.RequestHash(req)
					if err != nil {
						return err
					}
					id := uuid.NewString()
					token, err := issuer.IssueDelegatedUser(principal.DelegatedUserInput{Audience: "moderation", RPC: method, RequestID: id, RequestHash: hash, AccountID: uuid.NewString(), ProfileID: profiles[0], SessionEpoch: 1, AccountType: "regular", ClientExpiresAt: time.Now().Add(time.Minute)})
					if err != nil {
						return err
					}
					ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", "Bearer "+token, "x-request-id", id))
				}
			}
			return invoke(ctx, method, request, reply, conn, options...)
		}),
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

func withInternalModCtx(ctx context.Context, moderatorProfileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, "x-voice-internal", "true")
	return metadata.AppendToOutgoingContext(ctx, "x-voice-profile-id", moderatorProfileID.String())
}

// validateFixtureReports supplies explicit live owner decisions for tests of
// staff behavior. Production never infers this authority from the receipt.
func validateFixtureReports(t *testing.T, pool *pgxpool.Pool) {
	t.Helper()
	worker := &reportintake.Worker{Reports: &store.ReportStore{Pool: pool}, Targets: reportTargetFunc(func(_ context.Context, _ uuid.UUID, kind string, _ uuid.UUID) (string, uuid.UUID, error) {
		if kind == "user" {
			return "LIVE", uuid.New(), nil
		}
		return "LIVE", uuid.Nil, nil
	})}
	for {
		worked, err := worker.ValidateOne(context.Background())
		require.NoError(t, err)
		if !worked {
			return
		}
	}
}
