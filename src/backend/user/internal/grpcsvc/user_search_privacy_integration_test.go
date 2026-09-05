package grpcsvc

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/privacy"
	"voice/backend/user/internal/authctx"
	"voice/backend/user/internal/store"

	commonv1 "voice.app/voice/common/v1"
	userv1 "voice.app/voice/user/v1"
)

type searchPrivacyGraph struct {
	friends map[string]bool
	fof     map[string]bool
	err     error
}

func (g searchPrivacyGraph) AreFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.friends[privacyPairKey(a, b)], nil
}

func (g searchPrivacyGraph) AreFriendsOfFriends(_ context.Context, a, b uuid.UUID) (bool, error) {
	if g.err != nil {
		return false, g.err
	}
	return g.fof[privacyPairKey(a, b)], nil
}

func TestSearchProfiles_EnforcesFriendRequestDiscoverability(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	activeProfile := uuid.New()
	insertSearchPrivacyProfile(t, ctx, pool, viewerProfile, viewerAccount, "viewerprimary", "0001", true)
	insertSearchPrivacyProfile(t, ctx, pool, activeProfile, viewerAccount, "vieweractive", "0002", false)

	targets := map[string]struct {
		account  uuid.UUID
		profile  uuid.UUID
		audience privacy.Audience
	}{
		"everyone":  {uuid.New(), uuid.New(), privacy.EveryoneWithGuests()},
		"friend":    {uuid.New(), uuid.New(), privacy.FriendsOnly()},
		"fof":       {uuid.New(), uuid.New(), privacy.Audience{FriendsOfFriends: true}},
		"fofdirect": {uuid.New(), uuid.New(), privacy.Audience{FriendsOfFriends: true}},
		"nobody":    {uuid.New(), uuid.New(), privacy.Nobody()},
		"phone":     {uuid.New(), uuid.New(), privacy.EveryoneWithGuests()},
		"default":   {uuid.New(), uuid.New(), privacy.EveryoneWithGuests()},
		"blocked":   {uuid.New(), uuid.New(), privacy.EveryoneWithGuests()},
	}
	for name, target := range targets {
		insertSearchPrivacyProfile(t, ctx, pool, target.profile, target.account, "discover"+name, "1000", true)
	}

	privacyStore := store.NewPrivacyStore(pool)
	for name, target := range targets {
		if name == "default" {
			continue // Missing rows use the canonical default, without a write on search.
		}
		row := store.PrivacyRowFromSettings(target.profile, privacy.SettingsForPreset("gaming"))
		row.AllowFriendRequests = target.audience
		if name == "phone" {
			row.AllowPhoneSearch = privacy.Nobody()
		}
		_, err := privacyStore.Upsert(ctx, row)
		require.NoError(t, err)
	}

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	graph := searchPrivacyGraph{
		friends: map[string]bool{
			privacyPairKey(viewerProfile, targets["friend"].profile):    true,
			privacyPairKey(viewerProfile, targets["fofdirect"].profile): true,
		},
		fof: map[string]bool{privacyPairKey(viewerProfile, targets["fof"].profile): true},
	}
	blocker := &testBlockChecker{}
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb,
		func(s *UserGRPC) {
			s.SocialGraph = graph
			s.Blocks = blocker
		},
	)

	t.Run("active profile metadata controls relationship audience", func(t *testing.T) {
		resp, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, activeProfile), &userv1.SearchProfilesRequest{Query: "discover"})
		require.NoError(t, err)
		ids := collectProfileIDs(resp.GetProfileList().GetProfiles())
		require.Contains(t, ids, targets["everyone"].profile.String())
		require.Contains(t, ids, targets["phone"].profile.String(), "phone-search privacy must not affect text search")
		require.Contains(t, ids, targets["default"].profile.String(), "missing privacy rows use canonical gaming defaults")
		require.NotContains(t, ids, targets["friend"].profile.String())
		require.NotContains(t, ids, targets["fof"].profile.String())
		require.NotContains(t, ids, targets["nobody"].profile.String())
	})

	t.Run("primary profile is the fallback when active metadata is absent", func(t *testing.T) {
		caller := metadata.AppendToOutgoingContext(ctx, authctx.HeaderUserID, viewerAccount.String())
		resp, err := cli.SearchProfiles(caller, &userv1.SearchProfilesRequest{Query: "discover"})
		require.NoError(t, err)
		ids := collectProfileIDs(resp.GetProfileList().GetProfiles())
		require.Contains(t, ids, targets["friend"].profile.String())
		require.Contains(t, ids, targets["fof"].profile.String())
		require.Contains(t, ids, targets["fofdirect"].profile.String(), "a direct friend is included in friends-of-friends audience")
		require.NotContains(t, ids, targets["nobody"].profile.String())
	})

	t.Run("either-direction account block hides otherwise discoverable target", func(t *testing.T) {
		for _, direction := range []string{"viewer blocks target", "target blocks viewer"} {
			t.Run(direction, func(t *testing.T) {
				// AccountPairBlocked is the Social S2S pairwise result, so either
				// underlying direction is represented as one denied account pair.
				blocker.fn = func(viewer, other uuid.UUID) bool {
					return viewer == viewerAccount && other == targets["blocked"].account
				}
				t.Cleanup(func() { blocker.fn = nil })
				resp, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{Query: "discoverblocked"})
				require.NoError(t, err)
				require.NotContains(t, collectProfileIDs(resp.GetProfileList().GetProfiles()), targets["blocked"].profile.String())
			})
		}
	})
}

func TestSearchProfiles_GuestAndDependencyFailuresFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	guestAccount, guestProfile := uuid.New(), uuid.New()
	openAccount, openProfile := uuid.New(), uuid.New()
	guestRestrictedAccount, guestRestrictedProfile := uuid.New(), uuid.New()
	restrictedAccount, restrictedProfile := uuid.New(), uuid.New()
	fofRestrictedAccount, fofRestrictedProfile := uuid.New(), uuid.New()
	insertSearchPrivacyProfile(t, ctx, pool, viewerProfile, viewerAccount, "viewer", "0001", true)
	insertSearchPrivacyProfile(t, ctx, pool, guestProfile, guestAccount, "guest", "0002", true)
	insertSearchPrivacyProfile(t, ctx, pool, openProfile, openAccount, "guestopen", "0003", true)
	insertSearchPrivacyProfile(t, ctx, pool, guestRestrictedProfile, guestRestrictedAccount, "guestfriendsonly", "0004", true)
	insertSearchPrivacyProfile(t, ctx, pool, restrictedProfile, restrictedAccount, "failedaudience", "0005", true)
	insertSearchPrivacyProfile(t, ctx, pool, fofRestrictedProfile, fofRestrictedAccount, "fofdependency", "0006", true)

	privacyStore := store.NewPrivacyStore(pool)
	open := store.PrivacyRowFromSettings(openProfile, privacy.SettingsForPreset("gaming"))
	open.AllowFriendRequests = privacy.EveryoneWithGuests()
	_, err := privacyStore.Upsert(ctx, open)
	require.NoError(t, err)
	guestRestricted := store.PrivacyRowFromSettings(guestRestrictedProfile, privacy.SettingsForPreset("personal"))
	guestRestricted.AllowFriendRequests = privacy.FriendsOnly()
	_, err = privacyStore.Upsert(ctx, guestRestricted)
	require.NoError(t, err)
	restricted := store.PrivacyRowFromSettings(restrictedProfile, privacy.SettingsForPreset("personal"))
	restricted.AllowFriendRequests = privacy.FriendsOnly()
	_, err = privacyStore.Upsert(ctx, restricted)
	require.NoError(t, err)
	fofRestricted := store.PrivacyRowFromSettings(fofRestrictedProfile, privacy.SettingsForPreset("personal"))
	fofRestricted.AllowFriendRequests = privacy.Audience{FriendsOfFriends: true}
	_, err = privacyStore.Upsert(ctx, fofRestricted)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb,
		func(s *UserGRPC) { s.SocialGraph = searchPrivacyGraph{err: errors.New("social unavailable")} },
	)

	t.Run("guest uses canonical audience only", func(t *testing.T) {
		resp, err := cli.SearchProfiles(withGuestUserAuthCtx(ctx, guestAccount, guestProfile), &userv1.SearchProfilesRequest{Query: "guest"})
		require.NoError(t, err)
		ids := collectProfileIDs(resp.GetProfileList().GetProfiles())
		require.Contains(t, ids, openProfile.String())
		require.NotContains(t, ids, guestRestrictedProfile.String())
		require.NotContains(t, ids, guestProfile.String(), "a guest must not discover a profile on its own account")
	})

	t.Run("restricted audience dependency errors do not disclose", func(t *testing.T) {
		resp, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{Query: "failedaudience"})
		require.NoError(t, err)
		require.NotContains(t, collectProfileIDs(resp.GetProfileList().GetProfiles()), restrictedProfile.String())
	})

	t.Run("friends-of-friends dependency errors do not disclose", func(t *testing.T) {
		resp, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{Query: "fofdependency"})
		require.NoError(t, err)
		require.NotContains(t, collectProfileIDs(resp.GetProfileList().GetProfiles()), fofRestrictedProfile.String())
	})
}

func TestSearchProfiles_PrivacyFilteringFillsCursorPage(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	visibleAccount, visibleProfile := uuid.New(), uuid.New()
	insertSearchPrivacyProfile(t, ctx, pool, viewerProfile, viewerAccount, "viewer", "0001", true)
	for i := 0; i < searchProfilesBatch+1; i++ {
		account, profile := uuid.New(), uuid.New()
		insertSearchPrivacyProfile(t, ctx, pool, profile, account, fmt.Sprintf("a%03dpageprivacy", i), "0002", true)
	}
	insertSearchPrivacyProfile(t, ctx, pool, visibleProfile, visibleAccount, "zvisiblepageprivacy", "0003", true)

	privacyStore := store.NewPrivacyStore(pool)
	rows, err := pool.Query(ctx, `SELECT id FROM profiles WHERE username LIKE 'a%pageprivacy'`)
	require.NoError(t, err)
	for rows.Next() {
		var profileID uuid.UUID
		require.NoError(t, rows.Scan(&profileID))
		row := store.PrivacyRowFromSettings(profileID, privacy.SettingsForPreset("personal"))
		row.AllowFriendRequests = privacy.Nobody()
		_, err = privacyStore.Upsert(ctx, row)
		require.NoError(t, err)
	}
	require.NoError(t, rows.Err())
	rows.Close()
	visible := store.PrivacyRowFromSettings(visibleProfile, privacy.SettingsForPreset("gaming"))
	visible.AllowFriendRequests = privacy.EveryoneWithGuests()
	_, err = privacyStore.Upsert(ctx, visible)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb)

	resp, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{
		Query: "pageprivacy",
		Page:  &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	require.Equal(t, []string{visibleProfile.String()}, collectProfileIDs(resp.GetProfileList().GetProfiles()))
	require.False(t, resp.GetPage().GetHasMore())
}

func TestSearchProfiles_CursorPreservesVerifiedRankingAfterPrivacyFiltering(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	verifiedAccount, verifiedProfile := uuid.New(), uuid.New()
	unverifiedAccount, unverifiedProfile := uuid.New(), uuid.New()
	insertSearchPrivacyProfile(t, ctx, pool, viewerProfile, viewerAccount, "viewer", "0001", true)
	insertSearchPrivacyProfile(t, ctx, pool, verifiedProfile, verifiedAccount, "zverifiedcursor", "0002", true)
	insertSearchPrivacyProfile(t, ctx, pool, unverifiedProfile, unverifiedAccount, "aunverifiedcursor", "0003", true)
	_, err := pool.Exec(ctx, `UPDATE profiles SET verification_type = 'personal' WHERE id = $1`, verifiedProfile)
	require.NoError(t, err)

	privacyStore := store.NewPrivacyStore(pool)
	for _, profileID := range []uuid.UUID{verifiedProfile, unverifiedProfile} {
		row := store.PrivacyRowFromSettings(profileID, privacy.SettingsForPreset("gaming"))
		row.AllowFriendRequests = privacy.EveryoneWithGuests()
		_, err = privacyStore.Upsert(ctx, row)
		require.NoError(t, err)
	}
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb)

	first, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{
		Query: "cursor",
		Page:  &commonv1.CursorPageRequest{PageSize: 1},
	})
	require.NoError(t, err)
	require.Equal(t, []string{verifiedProfile.String()}, collectProfileIDs(first.GetProfileList().GetProfiles()))
	require.True(t, first.GetPage().GetHasMore())

	second, err := cli.SearchProfiles(withUserAuthCtx(ctx, viewerAccount, viewerProfile), &userv1.SearchProfilesRequest{
		Query: "cursor",
		Page:  &commonv1.CursorPageRequest{Cursor: first.GetPage().GetNextCursor(), PageSize: 1},
	})
	require.NoError(t, err)
	require.Equal(t, []string{unverifiedProfile.String()}, collectProfileIDs(second.GetProfileList().GetProfiles()))
	require.False(t, second.GetPage().GetHasMore())
}

func insertSearchPrivacyProfile(t *testing.T, ctx context.Context, pool *pgxpool.Pool, profileID, accountID uuid.UUID, username, discriminator string, primary bool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, $3, $4, $5, $6)`, profileID, accountID, username, discriminator, username, primary)
	require.NoError(t, err)
}
