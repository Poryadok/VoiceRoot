package testintegration

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	_ "voice/backend/pkg/integrationtest"
	"voice/backend/pkg/integrationtest"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/social/testsocial"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/grpcsvc"
	"voice/backend/user/internal/store"

	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	// .../user/testintegration/*.go -> repo root is 4 parents up from this dir
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", ".."))
}

func startPostgresPool(t *testing.T, ctx context.Context, database string) *pgxpool.Pool {
	t.Helper()
	return integrationtest.StartPostgres(t, ctx, database, "")
}

func withOutgoingAccountID(ctx context.Context, accountID uuid.UUID) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
}

type stubAvatarPresigner struct{}

func (stubAvatarPresigner) PresignPut(_ context.Context, objectKey, contentType string, contentLength int64) (string, map[string]string, time.Time, error) {
	_ = objectKey
	_ = contentLength
	return "https://r2.example/presigned-put", map[string]string{"Content-Type": contentType}, time.Now().Add(10 * time.Minute), nil
}

func collectProfileIDs(ps []*userv1.Profile) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.GetId())
	}
	return out
}

// TestSearchProfiles_UserSocialIntegration verifies PLAN Phase 1: UserService.SearchProfiles
// with real Social IsBlocked S2S — blocked accounts are omitted from discovery (both directions).
func TestSearchProfiles_UserSocialIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()

	userPool := startPostgresPool(t, ctx, "userdb")
	userMigration := filepath.Join(repoRoot(t), "src", "backend", "migrations", "user_db", "000001_init.up.sql")
	sqlUser, err := os.ReadFile(userMigration)
	require.NoError(t, err)
	_, err = userPool.Exec(ctx, string(sqlUser))
	require.NoError(t, err)

	socialPool := startPostgresPool(t, ctx, "socialdb")
	socialMigration := filepath.Join(repoRoot(t), "src", "backend", "migrations", "social_db", "000001_init.up.sql")
	sqlSocial, err := os.ReadFile(socialMigration)
	require.NoError(t, err)
	_, err = socialPool.Exec(ctx, string(sqlSocial))
	require.NoError(t, err)

	socialConn, stopSocial := testsocial.NewBufconnClient(t, socialPool)
	t.Cleanup(stopSocial)
	socialCli := socialv1.NewSocialServiceClient(socialConn)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	const bufSize = 1 << 20
	userLis := bufconn.Listen(bufSize)
	t.Cleanup(func() { _ = userLis.Close() })
	userGRPCSrv := grpc.NewServer()
	userv1.RegisterUserServiceServer(userGRPCSrv, &grpcsvc.UserGRPC{
		Profiles:            store.NewProfileStore(userPool),
		Presence:            store.NewPresenceStore(rdb),
		Blocks:              grpcsvc.NewSocialGRPCBlocks(socialConn),
		AvatarPresigner:     stubAvatarPresigner{},
		AvatarPublicBaseURL: "https://cdn-test.example",
	})
	go func() { _ = userGRPCSrv.Serve(userLis) }()
	t.Cleanup(userGRPCSrv.Stop)

	userConn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return userLis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = userConn.Close() })
	userCli := userv1.NewUserServiceClient(userConn)

	accountViewer := uuid.New()
	accountTarget := uuid.New()
	targetPID := uuid.New()
	queryToken := "Phase1SearchTokenXy9"

	_, err = userPool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'ph1find', '0707', $3, true)`,
		targetPID, accountTarget, "Display "+queryToken+" ZZ")
	require.NoError(t, err)

	mdViewer := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountViewer.String())

	t.Run("finds target before any block", func(t *testing.T) {
		resp, err := userCli.SearchProfiles(mdViewer, &userv1.SearchProfilesRequest{Query: queryToken})
		require.NoError(t, err)
		ids := collectProfileIDs(resp.GetProfileList().GetProfiles())
		require.Contains(t, ids, targetPID.String())
	})

	t.Run("hides target when viewer blocked target", func(t *testing.T) {
		_, err := socialCli.BlockAccount(withOutgoingAccountID(ctx, accountViewer), &socialv1.BlockAccountRequest{
			BlockedAccountId: accountTarget.String(),
		})
		require.NoError(t, err)

		resp, err := userCli.SearchProfiles(mdViewer, &userv1.SearchProfilesRequest{Query: queryToken})
		require.NoError(t, err)
		for _, p := range resp.GetProfileList().GetProfiles() {
			require.NotEqual(t, targetPID.String(), p.GetId(), "blocked profile must not appear in SearchProfiles")
		}

		_, err = socialCli.UnblockAccount(withOutgoingAccountID(ctx, accountViewer), &socialv1.UnblockAccountRequest{
			BlockedAccountId: accountTarget.String(),
		})
		require.NoError(t, err)
	})

	t.Run("finds target again after unblock", func(t *testing.T) {
		resp, err := userCli.SearchProfiles(mdViewer, &userv1.SearchProfilesRequest{Query: queryToken})
		require.NoError(t, err)
		ids := collectProfileIDs(resp.GetProfileList().GetProfiles())
		require.Contains(t, ids, targetPID.String())
	})

	t.Run("hides target when target blocked viewer", func(t *testing.T) {
		_, err := socialCli.BlockAccount(withOutgoingAccountID(ctx, accountTarget), &socialv1.BlockAccountRequest{
			BlockedAccountId: accountViewer.String(),
		})
		require.NoError(t, err)

		resp, err := userCli.SearchProfiles(mdViewer, &userv1.SearchProfilesRequest{Query: queryToken})
		require.NoError(t, err)
		for _, p := range resp.GetProfileList().GetProfiles() {
			require.NotEqual(t, targetPID.String(), p.GetId(), "reverse block must hide profile from viewer search")
		}

		_, err = socialCli.UnblockAccount(withOutgoingAccountID(ctx, accountTarget), &socialv1.UnblockAccountRequest{
			BlockedAccountId: accountViewer.String(),
		})
		require.NoError(t, err)
	})
}
