package grpcsvc

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func applyUserMigrationsForSubscriptionTests(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := repoRoot(t)
	for _, name := range []string{"000001_init.up.sql", "000003_profile_subscription.up.sql", "000004_profiles_verification.up.sql"} {
		sqlBytes, err := os.ReadFile(filepath.Join(root, "src", "backend", "migrations", "user_db", name))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func startUserPostgresForSubscriptionTests(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserMigrationsForSubscriptionTests(t, ctx, pool)
	return pool
}

func startUserGRPCForSubscriptionTests(t *testing.T, profiles *store.ProfileStore) (userv1.UserServiceClient, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, &UserGRPC{
		Profiles:            profiles,
		Presence:            store.NewPresenceStore(rdb),
		AvatarPresigner:     stubAvatarPresigner{},
		AvatarPublicBaseURL: "https://cdn-test.example",
	})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn), func() {}
}

func withAccountTier(ctx context.Context, accountID uuid.UUID, tier string) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, "x-voice-subscription-tier", tier)
}

// TestCreateProfile_FreeThirdProfileRejected documents free tier allows max 2 profiles.
func TestCreateProfile_FreeThirdProfileRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForSubscriptionTests(t, profiles)

	accountID := uuid.New()
	authed := withAccountTier(ctx, accountID, "free")

	_, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "Alt One"})
	require.NoError(t, err)
	_, err = cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "Alt Two"})
	require.NoError(t, err)

	_, err = cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "Alt Three"})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestCreateProfile_PremiumFifthProfileAccepted documents premium tier allows up to 5 profiles.
func TestCreateProfile_PremiumFifthProfileAccepted(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForSubscriptionTests(t, profiles)

	accountID := uuid.New()
	authed := withAccountTier(ctx, accountID, "premium")

	for i := 1; i <= 5; i++ {
		_, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{
			DisplayName: "Premium Alt " + string(rune('0'+i)),
		})
		require.NoError(t, err, "profile %d", i)
	}
	_, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{DisplayName: "Premium Sixth"})
	require.Error(t, err)
	require.Equal(t, codes.ResourceExhausted, status.Code(err))
}

// TestCreateAvatarPresignedUpload_FreeGifRejected documents animated avatars require Premium.
func TestCreateAvatarPresignedUpload_FreeGifRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForSubscriptionTests(t, profiles)

	accountID := uuid.New()
	pid := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'gifuser', '0001', 'Gif User', true)`,
		pid, accountID)
	require.NoError(t, err)

	authed := withAccountTier(ctx, accountID, "free")
	_, err = cli.CreateAvatarPresignedUpload(authed, &userv1.CreateAvatarPresignedUploadRequest{
		ProfileId:     pid.String(),
		ContentType:   "image/gif",
		ContentLength: 4096,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestUpdateProfile_FreeBannerRejected documents profile banners require Premium.
func TestUpdateProfile_FreeBannerRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForSubscriptionTests(t, profiles)

	accountID := uuid.New()
	pid := uuid.New()
	_, err := pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'banner', '0001', 'Banner User', true)`,
		pid, accountID)
	require.NoError(t, err)

	banner := "https://cdn-test.example/banners/" + pid.String() + ".png"
	authed := withAccountTier(ctx, accountID, "free")
	_, err = cli.UpdateProfile(authed, &userv1.UpdateProfileRequest{
		ProfileId: pid.String(),
		BannerUrl: &banner,
	})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}

// TestDowngrade_FreezesExcessProfiles documents downgrade freezes profiles beyond free cap.
func TestDowngrade_FreezesExcessProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	cli, _ := startUserGRPCForSubscriptionTests(t, profiles)

	accountID := uuid.New()
	authedPremium := withAccountTier(ctx, accountID, "premium")
	created := make([]string, 0, 3)
	for i := 1; i <= 3; i++ {
		resp, err := cli.CreateProfile(authedPremium, &userv1.CreateProfileRequest{
			DisplayName: "Keep or Freeze " + string(rune('0'+i)),
		})
		require.NoError(t, err)
		created = append(created, resp.GetProfile().GetId())
	}

	// Downgrade to free with 3 profiles: switching to a frozen profile must be blocked.
	authedFree := withAccountTier(ctx, accountID, "free")
	_, err := cli.SwitchProfile(authedFree, &userv1.SwitchProfileRequest{ProfileId: created[2]})
	require.Error(t, err)
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
}
