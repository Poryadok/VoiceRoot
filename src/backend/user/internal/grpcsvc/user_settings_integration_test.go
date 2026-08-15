package grpcsvc

import (
	"context"
	"net"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

func withInternalUserCtx(ctx context.Context) context.Context {
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "auth")
}

func startUserSettingsTestServer(t *testing.T, profiles *store.ProfileStore, privacy *store.PrivacyStore) userv1.UserServiceClient {
	t.Helper()
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, &UserGRPC{
		Profiles: profiles,
		Privacy:  privacy,
		Presence: store.NewPresenceStore(rdb),
	})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return userv1.NewUserServiceClient(conn)
}

func TestEnsurePrimaryProfile_Idempotent(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	accountID := uuid.New()
	internal := withInternalUserCtx(ctx)

	first, err := cli.EnsurePrimaryProfile(internal, &userv1.EnsurePrimaryProfileRequest{
		AccountId:   accountID.String(),
		DisplayHint: "player_one",
	})
	require.NoError(t, err)
	require.NotEmpty(t, first.GetProfile().GetId())
	require.True(t, first.GetProfile().GetIsPrimary())

	second, err := cli.EnsurePrimaryProfile(internal, &userv1.EnsurePrimaryProfileRequest{
		AccountId: accountID.String(),
	})
	require.NoError(t, err)
	require.Equal(t, first.GetProfile().GetId(), second.GetProfile().GetId())
}

func TestGetSettings_UpdateSettings_OwnedProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	accountID := uuid.New()
	internal := withInternalUserCtx(ctx)
	created, err := cli.EnsurePrimaryProfile(internal, &userv1.EnsurePrimaryProfileRequest{
		AccountId:   accountID.String(),
		DisplayHint: "settings_user",
	})
	require.NoError(t, err)
	profileID := created.GetProfile().GetId()

	userCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())

	got, err := cli.GetSettings(userCtx, &userv1.GetSettingsRequest{ProfileId: profileID})
	require.NoError(t, err)
	require.Equal(t, profileID, got.GetUserSettings().GetProfileId())
	require.Equal(t, "{}", got.GetUserSettings().GetNotificationPrefsJson())

	updated, err := cli.UpdateSettings(userCtx, &userv1.UpdateSettingsRequest{
		ProfileId: profileID,
		Settings: &userv1.UserSettings{
			ProfileId:             profileID,
			Language:              "en",
			Theme:                 "dark",
			NotificationPrefsJson: `{"dm":true}`,
		},
	})
	require.NoError(t, err)
	require.Equal(t, "en", updated.GetUserSettings().GetLanguage())
	require.Equal(t, "dark", updated.GetUserSettings().GetTheme())
	require.JSONEq(t, `{"dm":true}`, updated.GetUserSettings().GetNotificationPrefsJson())
}

func TestGetPrivacySettings_RejectsForeignProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	ownerAccount := uuid.New()
	otherAccount := uuid.New()
	internal := withInternalUserCtx(ctx)

	ownerProfile, err := cli.EnsurePrimaryProfile(internal, &userv1.EnsurePrimaryProfileRequest{
		AccountId: ownerAccount.String(),
	})
	require.NoError(t, err)
	otherProfile, err := cli.EnsurePrimaryProfile(internal, &userv1.EnsurePrimaryProfileRequest{
		AccountId: otherAccount.String(),
	})
	require.NoError(t, err)

	otherCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, otherAccount.String())
	_, err = cli.GetPrivacySettings(otherCtx, &userv1.GetPrivacySettingsRequest{
		ProfileId: ownerProfile.GetProfile().GetId(),
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	ownerCtx := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, ownerAccount.String())
	resp, err := cli.GetPrivacySettings(ownerCtx, &userv1.GetPrivacySettingsRequest{
		ProfileId: ownerProfile.GetProfile().GetId(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetPrivacySettings().GetPreset())
	_ = otherProfile
}

func TestGetPrivacySettings_InternalCaller_ReadsForeignProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	ownerAccount := uuid.New()
	internal := withInternalUserCtx(ctx)
	ownerProfile, err := cli.EnsurePrimaryProfile(internal, &userv1.EnsurePrimaryProfileRequest{
		AccountId: ownerAccount.String(),
	})
	require.NoError(t, err)

	// Social/Chat S2S must read target privacy without owning the profile.
	s2s := metadata.AppendToOutgoingContext(ctx, authctx.HeaderInternalCaller, "social")
	resp, err := cli.GetPrivacySettings(s2s, &userv1.GetPrivacySettingsRequest{
		ProfileId: ownerProfile.GetProfile().GetId(),
	})
	require.NoError(t, err)
	require.NotNil(t, resp.GetPrivacySettings())
}
