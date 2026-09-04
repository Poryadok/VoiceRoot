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

// TestResolvePrimaryProfileIDs_ResolvesOnlyExistingPrimaryProfiles documents the
// Auth S2S lookup used to join auth-owned phone hashes to User-owned primary
// profile ids. Missing accounts and accounts without a primary profile must not
// be materialized or included in the result.
func TestResolvePrimaryProfileIDs_ResolvesOnlyExistingPrimaryProfiles(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	primaryAccountID := uuid.New()
	primary, err := cli.EnsurePrimaryProfile(withInternalUserCtx(ctx), &userv1.EnsurePrimaryProfileRequest{
		AccountId: primaryAccountID.String(),
	})
	require.NoError(t, err)

	noPrimaryAccountID := uuid.New()
	_, err = pool.Exec(ctx, `
		INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
		VALUES ($1, $2, 'secondaryonly', '0001', 'Secondary Only', false)`,
		uuid.New(), noPrimaryAccountID)
	require.NoError(t, err)

	deletedPrimaryAccountID := uuid.New()
	deletedPrimary, err := cli.EnsurePrimaryProfile(withInternalUserCtx(ctx), &userv1.EnsurePrimaryProfileRequest{
		AccountId: deletedPrimaryAccountID.String(),
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at = now() WHERE id = $1`, uuid.MustParse(deletedPrimary.GetProfile().GetId()))
	require.NoError(t, err)

	frozenPrimaryAccountID := uuid.New()
	frozenPrimary, err := cli.EnsurePrimaryProfile(withInternalUserCtx(ctx), &userv1.EnsurePrimaryProfileRequest{
		AccountId: frozenPrimaryAccountID.String(),
	})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET frozen_at = now() WHERE id = $1`, uuid.MustParse(frozenPrimary.GetProfile().GetId()))
	require.NoError(t, err)

	absentAccountID := uuid.New()
	resp, err := cli.ResolvePrimaryProfileIDs(withInternalUserCtx(ctx), &userv1.ResolvePrimaryProfileIDsRequest{
		AccountIds: []string{
			primaryAccountID.String(), noPrimaryAccountID.String(), deletedPrimaryAccountID.String(),
			frozenPrimaryAccountID.String(), absentAccountID.String(),
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		primaryAccountID.String():       primary.GetProfile().GetId(),
		frozenPrimaryAccountID.String(): frozenPrimary.GetProfile().GetId(),
	}, resp.GetPrimaryProfileIds())

	var absentProfiles int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE account_id = $1`, absentAccountID).Scan(&absentProfiles)
	require.NoError(t, err)
	require.Zero(t, absentProfiles, "read-only lookup must not materialize an absent account")
}

func TestResolvePrimaryProfileIDs_RejectsNonInternalAndMalformedAccountID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	_, err := cli.ResolvePrimaryProfileIDs(ctx, &userv1.ResolvePrimaryProfileIDsRequest{
		AccountIds: []string{uuid.NewString()},
	})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = cli.ResolvePrimaryProfileIDs(withInternalUserCtx(ctx), &userv1.ResolvePrimaryProfileIDsRequest{
		AccountIds: []string{"not-a-uuid"},
	})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
}

// TestMarkAccountRegular_ClearsGuestFlagForEveryOwnedProfile documents the
// User-owned half of Auth's guest-to-regular conversion. The account remains
// the same; every profile, including a soft-deleted one, must stop being
// classified as a guest.
func TestMarkAccountRegular_ClearsGuestFlagForEveryOwnedProfile(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	accountID := uuid.New()
	primary, err := cli.EnsurePrimaryProfile(withInternalUserCtx(ctx), &userv1.EnsurePrimaryProfileRequest{
		AccountId:      accountID.String(),
		IsGuestAccount: true,
	})
	require.NoError(t, err)
	secondary, err := profiles.CreateSecondaryProfile(ctx, accountID, "Guest Alt", nil, nil)
	require.NoError(t, err)
	deleted, err := profiles.CreateSecondaryProfile(ctx, accountID, "Deleted Guest Alt", nil, nil)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET deleted_at = now() WHERE id = $1`, deleted.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE profiles SET is_guest_account = true WHERE account_id = $1`, accountID)
	require.NoError(t, err)

	_, err = cli.MarkAccountRegular(withInternalUserCtx(ctx), &userv1.MarkAccountRegularRequest{
		AccountId: accountID.String(),
	})
	require.NoError(t, err)

	var guestProfiles int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE account_id = $1 AND is_guest_account = true`, accountID).Scan(&guestProfiles)
	require.NoError(t, err)
	require.Zero(t, guestProfiles)

	rows, err := profiles.ListByAccountID(ctx, accountID)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.ElementsMatch(t, []uuid.UUID{uuid.MustParse(primary.GetProfile().GetId()), secondary.ID}, []uuid.UUID{rows[0].ID, rows[1].ID})
	for _, row := range rows {
		require.False(t, row.IsGuestAccount)
	}

	_, err = cli.MarkAccountRegular(withInternalUserCtx(ctx), &userv1.MarkAccountRegularRequest{
		AccountId: accountID.String(),
	})
	require.NoError(t, err, "repeated guest-to-regular conversion must be idempotent")
	unknownAccountID := uuid.New()
	_, err = cli.MarkAccountRegular(withInternalUserCtx(ctx), &userv1.MarkAccountRegularRequest{
		AccountId: unknownAccountID.String(),
	})
	require.NoError(t, err, "converting an account without profiles must succeed unchanged")

	var unknownProfiles int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM profiles WHERE account_id = $1`, unknownAccountID).Scan(&unknownProfiles)
	require.NoError(t, err)
	require.Zero(t, unknownProfiles, "marking an unknown account regular must not materialize profiles")
}

func TestMarkAccountRegular_RejectsNonInternalAndMalformedAccountID(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := startUserPostgresForSubscriptionTests(t, ctx)
	profiles := store.NewProfileStore(pool)
	privacy := store.NewPrivacyStore(pool)
	cli := startUserSettingsTestServer(t, profiles, privacy)

	_, err := cli.MarkAccountRegular(ctx, &userv1.MarkAccountRegularRequest{AccountId: uuid.NewString()})
	require.Error(t, err)
	require.Equal(t, codes.PermissionDenied, status.Code(err))

	_, err = cli.MarkAccountRegular(withInternalUserCtx(ctx), &userv1.MarkAccountRegularRequest{AccountId: "not-a-uuid"})
	require.Error(t, err)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
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
