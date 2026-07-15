package grpcsvc

import (
	"context"
	"net"
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
	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/profileaccent"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func startUserGRPCWithPrivacy(t *testing.T, profiles *store.ProfileStore, privacyStore *store.PrivacyStore) (userv1.UserServiceClient, func()) {
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
		Privacy:             privacyStore,
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

func startPostgresWithUserMigrations(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
	return pool
}

func authedAccount(ctx context.Context, accountID uuid.UUID, tier string) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, "x-voice-subscription-tier", tier)
}

// TestCreateProfile_GamingPresetBootstrapPrivacy documents preset applies privacy defaults on create.
func TestCreateProfile_GamingPresetBootstrapPrivacy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresWithUserMigrations(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacyStore := store.NewPrivacyStore(pool)
	cli, _ := startUserGRPCWithPrivacy(t, profiles, privacyStore)

	accountID := uuid.New()
	authed := authedAccount(ctx, accountID, "free")

	preset := "gaming"
	resp, err := cli.CreateProfile(authed, &userv1.CreateProfileRequest{
		DisplayName: "Gaming Persona",
		Preset:      &preset,
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetProfile().GetAccentColor())

	privResp, err := cli.GetPrivacySettings(authed, &userv1.GetPrivacySettingsRequest{
		ProfileId: resp.GetProfile().GetId(),
	})
	require.NoError(t, err)
	ps := privResp.GetPrivacySettings()
	require.Equal(t, "gaming", ps.GetPreset())
	require.True(t, privacy.FromProto(ps.GetShowOnline()).IsEveryoneShortcut())
}

// TestCreateProfile_InvalidPresetRejected documents unknown preset is rejected.
func TestCreateProfile_InvalidPresetRejected(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresWithUserMigrations(t, ctx)
	cli, _ := startUserGRPCWithPrivacy(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool))

	bad := "enterprise"
	_, err := cli.CreateProfile(authedAccount(ctx, uuid.New(), "free"), &userv1.CreateProfileRequest{
		DisplayName: "Bad Preset",
		Preset:      &bad,
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestCreateProfile_CustomAccentColor documents optional accent override on create.
func TestCreateProfile_CustomAccentColor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresWithUserMigrations(t, ctx)
	cli, _ := startUserGRPCWithPrivacy(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool))

	custom := "#AABBCC"
	resp, err := cli.CreateProfile(authedAccount(ctx, uuid.New(), "free"), &userv1.CreateProfileRequest{
		DisplayName: "Colored",
		AccentColor: &custom,
	})
	require.NoError(t, err)
	require.Equal(t, "#AABBCC", resp.GetProfile().GetAccentColor())
}

// TestCreateProfile_DefaultAccentFromPalette documents accent defaults from design token palette.
func TestCreateProfile_DefaultAccentFromPalette(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startPostgresWithUserMigrations(t, ctx)
	cli, _ := startUserGRPCWithPrivacy(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool))

	resp, err := cli.CreateProfile(authedAccount(ctx, uuid.New(), "free"), &userv1.CreateProfileRequest{
		DisplayName: "First Alt",
	})
	require.NoError(t, err)
	require.Equal(t, profileaccent.At(0), resp.GetProfile().GetAccentColor())
}
