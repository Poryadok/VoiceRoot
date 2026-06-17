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

func applyUserPrivacyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	root := repoRoot(t)
	for _, name := range []string{
		"000001_init.up.sql",
		"000002_privacy_settings.up.sql",
		"000003_profile_subscription.up.sql",
		"000004_phase13_profiles_verification.up.sql",
		"000005_privacy_guest_audience.up.sql",
	} {
		path := filepath.Join(root, "src", "backend", "migrations", "user_db", name)
		sqlBytes, err := os.ReadFile(path)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(sqlBytes))
		require.NoError(t, err)
	}
}

func startUserPrivacyTestServer(t *testing.T, pool *store.ProfileStore, privacy *store.PrivacyStore, rdb *redis.Client, opts ...func(*UserGRPC)) userv1.UserServiceClient {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	svc := &UserGRPC{
		Profiles: pool,
		Privacy:  privacy,
		Presence: store.NewPresenceStore(rdb),
	}
	for _, opt := range opts {
		opt(svc)
	}
	srv := grpc.NewServer()
	userv1.RegisterUserServiceServer(srv, svc)
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

type alwaysFriendsGraph struct{}

func (alwaysFriendsGraph) AreFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return true, nil
}

func (alwaysFriendsGraph) AreFriendsOfFriends(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, nil
}

type stubSocialGraph struct {
	friends map[string]bool
	fof     map[string]bool
}

func (s stubSocialGraph) AreFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if s.friends == nil {
		return false, nil
	}
	return s.friends[privacyPairKey(a, b)], nil
}

func (s stubSocialGraph) AreFriendsOfFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if s.fof == nil {
		return false, nil
	}
	return s.fof[privacyPairKey(a, b)], nil
}

func privacyPairKey(a, b uuid.UUID) string {
	if a.String() < b.String() {
		return a.String() + ":" + b.String()
	}
	return b.String() + ":" + a.String()
}

type stubProfileBlocks struct {
	blocked map[string]bool
}

func (s stubProfileBlocks) AccountPairBlocked(_ context.Context, viewer, other uuid.UUID) (bool, error) {
	if s.blocked == nil {
		return false, nil
	}
	return s.blocked[viewer.String()+":"+other.String()], nil
}

func withUserAuthCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, accountID.String())
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, profileID.String())
}

// TestGetPrivacySettings_GamingPresetDefaults documents gaming preset defaults from docs/features/privacy.md.
func TestGetPrivacySettings_GamingPresetDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	accountID := uuid.New()
	profileID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'gamer', '4242', 'Gamer', true)`,
		profileID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'gaming', 'everyone', 'everyone', 'everyone', 'nobody', 'everyone', 'everyone', 'everyone', true)`,
		profileID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	resp, err := cli.GetPrivacySettings(withUserAuthCtx(ctx, accountID, profileID), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	require.NoError(t, err)
	ps := resp.GetPrivacySettings()
	require.Equal(t, "gaming", ps.GetPreset())
	require.Equal(t, "everyone", ps.GetShowOnline())
	require.Equal(t, "everyone", ps.GetAllowDm())
	require.Equal(t, "nobody", ps.GetShowPhone())
}

// TestGetPrivacySettings_PersonalPresetDefaults documents personal preset DM audience defaults.
func TestGetPrivacySettings_PersonalPresetDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	accountID := uuid.New()
	profileID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'private', '1111', 'Private', true)`,
		profileID, accountID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'personal', 'friends', 'friends', 'friends_of_friends', 'nobody', 'friends_of_friends', 'friends_of_friends', 'everyone', false)`,
		profileID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	resp, err := cli.GetPrivacySettings(withUserAuthCtx(ctx, accountID, profileID), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	require.NoError(t, err)
	ps := resp.GetPrivacySettings()
	require.Equal(t, "personal", ps.GetPreset())
	require.Equal(t, "friends", ps.GetShowOnline())
	require.Equal(t, "friends_of_friends", ps.GetAllowDm())
}

// TestUpdatePrivacySettings_FriendsOnlyDM documents allow_dm=friends persists and round-trips.
func TestUpdatePrivacySettings_FriendsOnlyDM(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	accountID := uuid.New()
	profileID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'setter', '2222', 'Setter', true)`,
		profileID, accountID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	_, err = cli.UpdatePrivacySettings(withUserAuthCtx(ctx, accountID, profileID), &userv1.UpdatePrivacySettingsRequest{
		ProfileId: profileID.String(),
		Settings: &userv1.PrivacySettings{
			ProfileId:           profileID.String(),
			Preset:              "personal",
			ShowOnline:          "friends",
			ShowGameStatus:      "friends",
			ShowMmRating:        "friends",
			ShowPhone:           "nobody",
			ShowStories:         "friends",
			AllowDm:             "friends",
			AllowFriendRequests: "everyone",
			AllowGuestDm:        false,
		},
	})
	require.NoError(t, err)

	var allowDM string
	err = pool.QueryRow(ctx, `SELECT allow_dm FROM privacy_settings WHERE profile_id = $1`, profileID).Scan(&allowDM)
	require.NoError(t, err)
	require.Equal(t, "friends", allowDM)
}

func withGuestUserAuthCtx(ctx context.Context, accountID, profileID uuid.UUID) context.Context {
	ctx = withUserAuthCtx(ctx, accountID, profileID)
	return metadata.AppendToOutgoingContext(ctx, "x-voice-account-type", "guest")
}

// TestUpdatePrivacySettings_MultiselectIncludesGuestAccounts documents privacy.md audience multiselect
// must expose "guest accounts" as an explicit option (stored separately from friends/space members).
func TestUpdatePrivacySettings_MultiselectIncludesGuestAccounts(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	accountID := uuid.New()
	profileID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'privacy', '3333', 'Privacy', true)`,
		profileID, accountID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	_, err = pool.Exec(ctx, `
ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_online_include_guests BOOLEAN NOT NULL DEFAULT false`)
	require.NoError(t, err)

	_, err = cli.UpdatePrivacySettings(withUserAuthCtx(ctx, accountID, profileID), &userv1.UpdatePrivacySettingsRequest{
		ProfileId: profileID.String(),
		Settings: &userv1.PrivacySettings{
			ProfileId:           profileID.String(),
			Preset:              "personal",
			ShowOnline:          "friends",
			ShowGameStatus:      "friends",
			ShowMmRating:        "friends",
			ShowPhone:           "nobody",
			ShowStories:         "friends",
			AllowDm:             "friends",
			AllowFriendRequests: "everyone",
			AllowGuestDm:        false,
		},
	})
	require.NoError(t, err)

	// Explicit guest-audience selection (multiselect) must round-trip once API lands.
	_, err = pool.Exec(ctx, `
UPDATE privacy_settings SET show_online_include_guests = true WHERE profile_id = $1`, profileID)
	require.NoError(t, err)

	var includeGuests bool
	err = pool.QueryRow(ctx, `
SELECT show_online_include_guests FROM privacy_settings WHERE profile_id = $1`, profileID).Scan(&includeGuests)
	require.NoError(t, err)
	require.True(t, includeGuests, "guest audience selection must persist for multiselect")

	resp, err := cli.GetPrivacySettings(withUserAuthCtx(ctx, accountID, profileID), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	require.NoError(t, err)
	field := resp.GetPrivacySettings().ProtoReflect().Descriptor().Fields().ByName("show_online_include_guests")
	require.NotNil(t, field, "PrivacySettings proto must expose guest audience selection")
	require.True(t, resp.GetPrivacySettings().ProtoReflect().Get(field).Bool())
}

// TestGetPresence_GuestViewer_HiddenWhenGuestAudienceExcluded documents privacy.md:
// guests do not see online status unless "guest accounts" is in the audience.
func TestGetPresence_GuestViewer_HiddenWhenGuestAudienceExcluded(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	guestAccount := uuid.New()
	guestProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'owner', '4444', 'Owner', true),
       ($3, $4, 'guestview', '5555', 'Guest Viewer', true)`,
		ownerProfile, ownerAccount, guestProfile, guestAccount)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'personal', 'friends', 'friends', 'friends', 'nobody', 'friends', 'friends', 'everyone', false)`,
		ownerProfile)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	_, err = cli.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{
		Status: "online",
	})
	require.NoError(t, err)

	resp, err := cli.GetPresence(withGuestUserAuthCtx(ctx, guestAccount, guestProfile), &userv1.GetPresenceRequest{
		ProfileId: ownerProfile.String(),
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetPresenceStatus().GetStatus(),
		"guest viewer must not see online status when guest audience is excluded")
}

// TestGetPrivacySettings_NewProfile_BootstrapPreset documents primary profile gets default preset row on first read.
func TestGetPrivacySettings_NewProfile_BootstrapPreset(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	accountID := uuid.New()
	profileID := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'newbie', '3333', 'Newbie', true)`,
		profileID, accountID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	resp, err := cli.GetPrivacySettings(withUserAuthCtx(ctx, accountID, profileID), &userv1.GetPrivacySettingsRequest{
		ProfileId: profileID.String(),
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.GetPrivacySettings().GetPreset())
}

// TestGetPresence_FriendsOnly_HiddenFromStranger documents show_online=friends hides live status from non-friends.
func TestGetPresence_FriendsOnly_HiddenFromStranger(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	strangerAccount := uuid.New()
	strangerProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'owner', '4444', 'Owner', true),
       ($3, $4, 'stranger', '5555', 'Stranger', true)`,
		ownerProfile, ownerAccount, strangerProfile, strangerAccount)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'personal', 'friends', 'friends', 'friends', 'nobody', 'friends', 'friends', 'everyone', false)`,
		ownerProfile)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb,
		func(s *UserGRPC) { s.SocialGraph = stubSocialGraph{} },
	)

	_, err = cli.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{
		Status: "online",
	})
	require.NoError(t, err)

	resp, err := cli.GetPresence(withUserAuthCtx(ctx, strangerAccount, strangerProfile), &userv1.GetPresenceRequest{
		ProfileId: ownerProfile.String(),
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetPresenceStatus().GetStatus())
}

// TestGetPresence_Everyone_VisibleToAnyViewer documents show_online=everyone returns live status to any viewer.
func TestGetPresence_Everyone_VisibleToAnyViewer(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	friendAccount := uuid.New()
	friendProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'owner', '4444', 'Owner', true),
       ($3, $4, 'friend', '5555', 'Friend', true)`,
		ownerProfile, ownerAccount, friendProfile, friendAccount)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'personal', 'everyone', 'friends', 'friends', 'nobody', 'friends', 'friends', 'everyone', false)`,
		ownerProfile)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	_, err = cli.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{
		Status: "online",
	})
	require.NoError(t, err)

	resp, err := cli.GetPresence(withUserAuthCtx(ctx, friendAccount, friendProfile), &userv1.GetPresenceRequest{
		ProfileId: ownerProfile.String(),
	})
	require.NoError(t, err)
	require.Equal(t, "online", resp.GetPresenceStatus().GetStatus())
}

// TestGetPresence_FriendsOnly_VisibleToFriend documents show_online=friends allows friends to see status.
func TestGetPresence_FriendsOnly_VisibleToFriend(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	friendAccount := uuid.New()
	friendProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'owner', '4444', 'Owner', true),
       ($3, $4, 'friend', '5555', 'Friend', true)`,
		ownerProfile, ownerAccount, friendProfile, friendAccount)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'personal', 'friends', 'friends', 'friends', 'nobody', 'friends', 'friends', 'everyone', false)`,
		ownerProfile)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb,
		func(s *UserGRPC) { s.SocialGraph = alwaysFriendsGraph{} },
	)

	_, err = cli.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{
		Status: "online",
	})
	require.NoError(t, err)

	resp, err := cli.GetPresence(withUserAuthCtx(ctx, friendAccount, friendProfile), &userv1.GetPresenceRequest{
		ProfileId: ownerProfile.String(),
	})
	require.NoError(t, err)
	require.Equal(t, "online", resp.GetPresenceStatus().GetStatus())
}

// TestGetBulkPresence_FriendsOnly_FiltersStranger documents bulk presence applies same privacy rules.
func TestGetBulkPresence_FriendsOnly_FiltersStranger(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	strangerAccount := uuid.New()
	strangerProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'owner', '4444', 'Owner', true),
       ($3, $4, 'stranger', '5555', 'Stranger', true)`,
		ownerProfile, ownerAccount, strangerProfile, strangerAccount)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
INSERT INTO privacy_settings (profile_id, preset, show_online, show_game_status, show_mm_rating, show_phone, show_stories, allow_dm, allow_friend_requests, allow_guest_dm)
VALUES ($1, 'personal', 'friends', 'friends', 'friends', 'nobody', 'friends', 'friends', 'everyone', false)`,
		ownerProfile)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb,
		func(s *UserGRPC) { s.SocialGraph = stubSocialGraph{} },
	)

	_, err = cli.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{
		Status: "online",
	})
	require.NoError(t, err)

	resp, err := cli.GetBulkPresence(withUserAuthCtx(ctx, strangerAccount, strangerProfile), &userv1.GetBulkPresenceRequest{
		ProfileIds: []string{ownerProfile.String()},
	})
	require.NoError(t, err)
	require.Empty(t, resp.GetByProfileId()[ownerProfile.String()].GetStatus())
}

// TestGetProfile_BlockedPair_NotFound documents privacy.md block UX: opaque not found for blocked profiles.
func TestGetProfile_BlockedPair_NotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	viewerAccount := uuid.New()
	targetAccount := uuid.New()
	targetProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'blocked', '7777', 'Blocked', true)`,
		targetProfile, targetAccount)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb,
		func(s *UserGRPC) {
			s.Blocks = stubProfileBlocks{blocked: map[string]bool{viewerAccount.String() + ":" + targetAccount.String(): true}}
		},
	)

	_, err = cli.GetProfile(withUserAuthCtx(ctx, viewerAccount, uuid.New()), &userv1.GetProfileRequest{
		By: &userv1.GetProfileRequest_ProfileId{ProfileId: targetProfile.String()},
	})
	require.Error(t, err)
	require.Equal(t, codes.NotFound, status.Code(err))
}
