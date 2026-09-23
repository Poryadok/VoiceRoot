package grpcsvc

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
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
	"voice/backend/user/internal/store"

	socialv1 "voice.app/voice/social/v1"
	userv1 "voice.app/voice/user/v1"
)

// TestCustomStatus_OnlyPremiumOwnersMaySetNonEmpty_DurableAndPresence is the
// RED contract for the two public write paths.  A free owner may explicitly
// clear a value, but a rejected non-empty write must leave the previous value.
func TestCustomStatus_OnlyPremiumOwnersMaySetNonEmpty_DurableAndPresence(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))

	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, custom_status)
VALUES ($1, $2, 'statusowner', '7001', 'Status Owner', true, 'durable-before')`, profileID, accountID)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	free := withProfileTier(ctx, accountID, profileID, "free")
	nonEmpty := "durable-denied"
	_, err = cli.UpdateProfile(free, &userv1.UpdateProfileRequest{ProfileId: profileID.String(), CustomStatus: &nonEmpty})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	var durable string
	require.NoError(t, pool.QueryRow(ctx, `SELECT custom_status FROM profiles WHERE id = $1`, profileID).Scan(&durable))
	require.Equal(t, "durable-before", durable, "a denied durable write must not mutate PostgreSQL")

	premium := withProfileTier(ctx, accountID, profileID, "premium")
	_, err = cli.UpdateProfile(premium, &userv1.UpdateProfileRequest{ProfileId: profileID.String(), CustomStatus: &nonEmpty})
	require.NoError(t, err)
	_, err = cli.UpdateProfile(free, &userv1.UpdateProfileRequest{ProfileId: profileID.String(), CustomStatus: stringPtr("")})
	require.NoError(t, err, "a free owner may clear an existing durable value")
	require.NoError(t, pool.QueryRow(ctx, `SELECT custom_status FROM profiles WHERE id = $1`, profileID).Scan(&durable))
	require.Empty(t, durable)

	presenceBefore := "presence-before"
	_, err = cli.UpdatePresence(premium, &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &presenceBefore})
	require.NoError(t, err)
	presenceDenied := "presence-denied"
	_, err = cli.UpdatePresence(free, &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &presenceDenied})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	snapshot, err := store.NewPresenceStore(rdb).Get(ctx, profileID)
	require.NoError(t, err)
	require.Equal(t, presenceBefore, snapshot.CustomStatus, "a denied session write must not mutate Redis")

	_, err = cli.UpdatePresence(free, &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: stringPtr("")})
	require.NoError(t, err, "a free owner may clear an existing session value")
	snapshot, err = store.NewPresenceStore(rdb).Get(ctx, profileID)
	require.NoError(t, err)
	require.Empty(t, snapshot.CustomStatus)
}

// TestCustomStatus_DurableProfileProjectionIsOwnerOnly keeps profile reads from
// becoming a bypass around the viewer-aware Presence API.
func TestCustomStatus_DurableProfileProjectionIsOwnerOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))

	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	foreignAccount, foreignProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, custom_status)
VALUES ($1, $2, 'projectionowner', '7002', 'Owner', true, 'durable-secret'),
       ($3, $4, 'projectionforeign', '7003', 'Foreign', true, NULL)`,
		ownerProfile, ownerAccount, foreignProfile, foreignAccount)
	require.NoError(t, err)

	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb,
		func(s *UserGRPC) { s.Blocks = &testBlockChecker{} },
	)

	owner, err := cli.GetProfile(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: ownerProfile.String()}})
	require.NoError(t, err)
	require.Equal(t, "durable-secret", owner.GetProfile().GetCustomStatus())

	for name, readCtx := range map[string]context.Context{
		"foreign":    withUserAuthCtx(ctx, foreignAccount, foreignProfile),
		"viewerless": ctx,
	} {
		t.Run(name, func(t *testing.T) {
			one, err := cli.GetProfile(readCtx, &userv1.GetProfileRequest{By: &userv1.GetProfileRequest_ProfileId{ProfileId: ownerProfile.String()}})
			require.NoError(t, err)
			require.Nil(t, one.GetProfile().CustomStatus)

			many, err := cli.GetProfiles(readCtx, &userv1.GetProfilesRequest{ProfileIds: []string{ownerProfile.String()}})
			require.NoError(t, err)
			require.Len(t, many.GetProfileList().GetProfiles(), 1)
			require.Nil(t, many.GetProfileList().GetProfiles()[0].CustomStatus)
		})
	}

	search, err := cli.SearchProfiles(withUserAuthCtx(ctx, foreignAccount, foreignProfile), &userv1.SearchProfilesRequest{Query: "projectionowner"})
	require.NoError(t, err)
	require.NotEmpty(t, search.GetProfileList().GetProfiles())
	require.Nil(t, search.GetProfileList().GetProfiles()[0].CustomStatus,
		"SearchProfiles must use the public projection, never the owner-scoped durable custom status")
}

// TestCustomStatus_OwnerListMyProfilesRetainsDurableValue isolates the owner
// projection from GetProfile and public-list projections.
func TestCustomStatus_OwnerListMyProfilesRetainsDurableValue(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, custom_status)
VALUES ($1, $2, 'listowner', '7020', 'Owner', true, 'durable-list-secret')`, ownerProfile, ownerAccount)
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	owned, err := cli.ListMyProfiles(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.ListMyProfilesRequest{})
	require.NoError(t, err)
	require.Len(t, owned.GetProfileList().GetProfiles(), 1)
	require.Equal(t, "durable-list-secret", owned.GetProfileList().GetProfiles()[0].GetCustomStatus(),
		"the owner list projection must retain the durable premium custom status")
}

// Each case remains independent so one missing gate cannot mask the remaining
// acceptance contracts in a fail-fast integration test.
func TestCustomStatusWritePrecedenceAndPresenceOmission_RED(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
	ownerAccount, ownerProfile, foreignAccount, foreignProfile := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary, custom_status) VALUES
($1,$2,'precedenceowner','7011','Owner',true,'before'), ($3,$4,'precedenceforeign','7012','Foreign',true,NULL)`, ownerProfile, ownerAccount, foreignProfile, foreignAccount)
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

	t.Run("foreign nonempty stays opaque not found", func(t *testing.T) {
		value := "forbidden"
		_, err := cli.UpdateProfile(withProfileTier(ctx, foreignAccount, foreignProfile, "free"), &userv1.UpdateProfileRequest{ProfileId: ownerProfile.String(), CustomStatus: &value})
		require.Equal(t, codes.NotFound, status.Code(err))
	})
	t.Run("missing tier denies nonempty durable write", func(t *testing.T) {
		value := "forbidden"
		_, err := cli.UpdateProfile(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdateProfileRequest{ProfileId: ownerProfile.String(), CustomStatus: &value})
		require.Equal(t, codes.FailedPrecondition, status.Code(err))
	})
	t.Run("invalid presence input wins over entitlement", func(t *testing.T) {
		value := "forbidden"
		_, err := cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "free"), &userv1.UpdatePresenceRequest{Status: "invalid", CustomStatus: &value})
		require.Equal(t, codes.InvalidArgument, status.Code(err))
	})
	t.Run("omitted presence custom status preserves existing value", func(t *testing.T) {
		before := "session-before"
		_, err := cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &before})
		require.NoError(t, err)
		_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{Status: "idle"})
		require.NoError(t, err)
		snapshot, err := store.NewPresenceStore(rdb).Get(ctx, ownerProfile)
		require.NoError(t, err)
		require.Equal(t, before, snapshot.CustomStatus)
	})
}

// Denial must happen before either store write or event publication.  Keeping
// this separate from entitlement assertions ensures it runs on an unfixed tip.
func TestCustomStatusDeniedWrites_HaveNoEventsOrPresenceMutation_RED(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
	accountID, profileID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary) VALUES ($1,$2,'noeventowner','7013','Owner',true)`, profileID, accountID)
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	events := &customStatusEventsRecorder{}
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb, func(s *UserGRPC) { s.Events = events })

	seed := "before"
	_, err = cli.UpdatePresence(withProfileTier(ctx, accountID, profileID, "premium"), &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &seed})
	require.NoError(t, err)
	events.presence = 0
	presenceKey := "voice:user:presence:" + profileID.String()
	lastSeenKey := "voice:user:last_seen:" + profileID.String()
	// Make heartbeat refreshes independently observable even when the two RPCs
	// execute inside the same wall-clock second.
	require.NoError(t, rdb.HSet(ctx, presenceKey, "ts_unix", "1").Err())
	require.NoError(t, rdb.Set(ctx, lastSeenKey, "1", 30*24*time.Hour).Err())
	mr.FastForward(time.Minute)
	presenceBefore, err := rdb.HGetAll(ctx, presenceKey).Result()
	require.NoError(t, err)
	presenceTTLBefore, err := rdb.TTL(ctx, presenceKey).Result()
	require.NoError(t, err)
	lastSeenBefore, err := rdb.Get(ctx, lastSeenKey).Result()
	require.NoError(t, err)
	lastSeenTTLBefore, err := rdb.TTL(ctx, lastSeenKey).Result()
	require.NoError(t, err)
	denied := "denied"
	_, err = cli.UpdatePresence(withProfileTier(ctx, accountID, profileID, "free"), &userv1.UpdatePresenceRequest{Status: "idle", CustomStatus: &denied})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	snapshot, err := store.NewPresenceStore(rdb).Get(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, "online", snapshot.Status)
	assert.Equal(t, seed, snapshot.CustomStatus)
	presenceAfter, err := rdb.HGetAll(ctx, presenceKey).Result()
	require.NoError(t, err)
	presenceTTLAfter, err := rdb.TTL(ctx, presenceKey).Result()
	require.NoError(t, err)
	lastSeenAfter, err := rdb.Get(ctx, lastSeenKey).Result()
	require.NoError(t, err)
	lastSeenTTLAfter, err := rdb.TTL(ctx, lastSeenKey).Result()
	require.NoError(t, err)
	assert.Equal(t, presenceBefore, presenceAfter, "a denied write must not refresh the session timestamp or any presence hash field")
	assert.Equal(t, presenceTTLBefore, presenceTTLAfter, "a denied write must not refresh the session TTL")
	assert.Equal(t, lastSeenBefore, lastSeenAfter, "a denied write must not refresh last_seen")
	assert.Equal(t, lastSeenTTLBefore, lastSeenTTLAfter, "a denied write must not refresh the last_seen TTL")
	assert.Zero(t, events.presence)

	profiles := store.NewProfileStore(pool)
	durableBefore, err := profiles.GetByID(ctx, profileID)
	require.NoError(t, err)
	changedName := "Denied Name"
	_, err = cli.UpdateProfile(withProfileTier(ctx, accountID, profileID, "free"), &userv1.UpdateProfileRequest{
		ProfileId:    profileID.String(),
		DisplayName:  &changedName,
		CustomStatus: &denied,
	})
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
	durableAfter, err := profiles.GetByID(ctx, profileID)
	require.NoError(t, err)
	assert.Equal(t, durableBefore, durableAfter, "a denied custom-status write must leave the complete durable profile row unchanged")
	assert.Zero(t, events.profile)
}

// TestCustomStatus_PresenceBlocksSocialDirectionsFailClosed_RED binds User's
// sparse presence projection to the two directed Social IsBlocked calls.  The
// table keeps the outgoing and reverse Social answers independently executable.
func TestCustomStatus_PresenceBlocksSocialDirectionsFailClosed_RED(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	for _, tc := range []struct {
		name    string
		blocked func(viewerAccount, ownerAccount uuid.UUID) map[string]bool
	}{
		{
			name: "viewer blocks owner through outgoing Social IsBlocked",
			blocked: func(viewerAccount, ownerAccount uuid.UUID) map[string]bool {
				return map[string]bool{viewerAccount.String() + ":" + ownerAccount.String(): true}
			},
		},
		{
			name: "owner blocks viewer through reverse Social IsBlocked",
			blocked: func(viewerAccount, ownerAccount uuid.UUID) map[string]bool {
				return map[string]bool{ownerAccount.String() + ":" + viewerAccount.String(): true}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
			integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))
			ownerAccount, ownerProfile := uuid.New(), uuid.New()
			viewerAccount, viewerProfile := uuid.New(), uuid.New()
			_, err := pool.Exec(ctx, `INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary) VALUES
($1,$2,'socialblockowner','7021','Owner',true), ($3,$4,'socialblockviewer','7022','Viewer',true)`,
				ownerProfile, ownerAccount, viewerProfile, viewerAccount)
			require.NoError(t, err)
			privacyStore := store.NewPrivacyStore(pool)
			seedPrivacyPreset(ctx, t, privacyStore, ownerProfile, "personal")
			mr := miniredis.RunT(t)
			t.Cleanup(mr.Close)
			rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			blocks := startCustomStatusSocialBlocks(t, tc.blocked(viewerAccount, ownerAccount))
			cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb, func(s *UserGRPC) {
				s.SocialGraph = alwaysFriendsGraph{}
				s.Blocks = blocks
			})
			secret := "social-direction-secret"
			_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &secret})
			require.NoError(t, err)

			viewer := withUserAuthCtx(ctx, viewerAccount, viewerProfile)
			direct, err := cli.GetPresence(viewer, &userv1.GetPresenceRequest{ProfileId: ownerProfile.String()})
			require.NoError(t, err)
			assert.Empty(t, direct.GetPresenceStatus().GetStatus(), "direct presence must be sparse for either Social block direction")
			assert.Nil(t, direct.GetPresenceStatus().CustomStatus, "direct presence must be sparse for either Social block direction")
			bulk, err := cli.GetBulkPresence(viewer, &userv1.GetBulkPresenceRequest{ProfileIds: []string{ownerProfile.String()}})
			require.NoError(t, err)
			assert.Empty(t, bulk.GetByProfileId()[ownerProfile.String()].GetStatus(), "bulk presence must be sparse for either Social block direction")
			assert.Nil(t, bulk.GetByProfileId()[ownerProfile.String()].CustomStatus, "bulk presence must be sparse for either Social block direction")
		})
	}
}

type customStatusSocialBlocksServer struct {
	socialv1.UnimplementedSocialServiceServer
	blocked map[string]bool
}

func (s customStatusSocialBlocksServer) IsBlocked(_ context.Context, req *socialv1.IsBlockedRequest) (*socialv1.IsBlockedResponse, error) {
	return &socialv1.IsBlockedResponse{Blocked: s.blocked[req.GetAccountIdA()+":"+req.GetAccountIdB()]}, nil
}

func startCustomStatusSocialBlocks(t *testing.T, blocked map[string]bool) *SocialGRPCBlocks {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = lis.Close() })
	srv := grpc.NewServer()
	socialv1.RegisterSocialServiceServer(srv, customStatusSocialBlocksServer{blocked: blocked})
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	conn, err := grpc.NewClient("passthrough:///social-blocks",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return NewSocialGRPCBlocks(conn)
}

type customStatusEventsRecorder struct{ profile, presence int }

func (r *customStatusEventsRecorder) PublishProfileCreated(context.Context, string, string) error {
	return nil
}
func (r *customStatusEventsRecorder) PublishProfileUpdated(context.Context, string, []string) error {
	r.profile++
	return nil
}
func (r *customStatusEventsRecorder) PublishProfileSwitched(context.Context, string, string, string) error {
	return nil
}
func (r *customStatusEventsRecorder) PublishVerified(context.Context, string, string) error {
	return nil
}
func (r *customStatusEventsRecorder) PublishPresenceChanged(context.Context, string, string, string) error {
	r.presence++
	return nil
}

func (*customStatusEventsRecorder) PublishGameDetected(context.Context, string, string) error {
	return nil
}

func (*customStatusEventsRecorder) PublishSettingsChanged(context.Context, string, []string, string) error {
	return nil
}

// TestCustomStatus_PresenceAudienceAndBlockFailClosed applies the existing
// show_online decision to custom status for both direct and bulk presence.
func TestCustomStatus_PresenceAudienceAndBlockFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))

	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'presenceowner', '7004', 'Owner', true),
       ($3, $4, 'presenceviewer', '7005', 'Viewer', true)`,
		ownerProfile, ownerAccount, viewerProfile, viewerAccount)
	require.NoError(t, err)

	privacyStore := store.NewPrivacyStore(pool)
	seedPrivacyPreset(ctx, t, privacyStore, ownerProfile, "personal")
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	profiles := store.NewProfileStore(pool)
	cli := startUserPrivacyTestServer(t, profiles, privacyStore, rdb, func(s *UserGRPC) {
		s.SocialGraph = alwaysFriendsGraph{}
		s.Blocks = stubProfileBlocks{}
	})

	custom := "present-secret"
	_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &custom})
	require.NoError(t, err)

	// An allowed foreign viewer and self can see the session field.
	viewer := withUserAuthCtx(ctx, viewerAccount, viewerProfile)
	assertCustomStatusPresentForDirectAndBulk(t, cli, viewer, ownerProfile, custom)
	self, err := cli.GetPresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.GetPresenceRequest{ProfileId: ownerProfile.String()})
	require.NoError(t, err)
	require.Equal(t, custom, self.GetPresenceStatus().GetCustomStatus())

	blockedCli := startUserPrivacyTestServer(t, profiles, privacyStore, rdb, func(s *UserGRPC) {
		s.SocialGraph = alwaysFriendsGraph{}
		s.Blocks = stubProfileBlocks{blocked: map[string]bool{viewerAccount.String() + ":" + ownerAccount.String(): true}}
	})
	assertCustomStatusOmittedForDirectAndBulk(t, blockedCli, viewer, ownerProfile)

	// show_online=nobody, invisible, absent/ambiguous identity, missing privacy,
	// and dependency errors are sparse responses, never policy-reason disclosures.
	nobody := privacy.SettingsForPreset("personal")
	nobody.ShowOnline = privacy.Nobody()
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(ownerProfile, nobody))
	require.NoError(t, err)
	assertCustomStatusOmittedForDirectAndBulk(t, cli, viewer, ownerProfile)

	personal := privacy.SettingsForPreset("personal")
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(ownerProfile, personal))
	require.NoError(t, err)
	_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{Status: "invisible", CustomStatus: &custom})
	require.NoError(t, err)
	assertCustomStatusOmittedForDirectAndBulk(t, cli, viewer, ownerProfile)
	_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{Status: "online", CustomStatus: &custom})
	require.NoError(t, err)

	failingCli := startUserPrivacyTestServer(t, profiles, privacyStore, rdb, func(s *UserGRPC) { s.SocialGraph = lastSeenFailingSocialGraph{} })
	assertCustomStatusOmittedForDirectAndBulk(t, failingCli, viewer, ownerProfile)
	_, err = pool.Exec(ctx, `DELETE FROM privacy_settings WHERE profile_id = $1`, ownerProfile)
	require.NoError(t, err)
	assertCustomStatusOmittedForDirectAndBulk(t, cli, viewer, ownerProfile)
	assertCustomStatusOmittedForDirectAndBulk(t, cli, context.Background(), ownerProfile)
	ambiguous := metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, viewerProfile.String(), authctx.HeaderProfileID, ownerProfile.String())
	assertCustomStatusOmittedForDirectAndBulk(t, cli, ambiguous, ownerProfile)
}

// TestCustomStatus_PresenceMissingSocialBlockWiringFailsClosed_RED covers a
// valid deployment with optional SOCIAL_GRPC_ADDR omitted. A missing Social
// block checker is not evidence that an otherwise allowed foreign viewer may
// receive a live custom status. The owner remains able to read their own
// presence through both public shapes.
func TestCustomStatus_PresenceMissingSocialBlockWiringFailsClosed_RED(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))

	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'nilblocksowner', '7018', 'Owner', true),
       ($3, $4, 'nilblocksviewer', '7019', 'Viewer', true)`,
		ownerProfile, ownerAccount, viewerProfile, viewerAccount)
	require.NoError(t, err)

	privacyStore := store.NewPrivacyStore(pool)
	seedPrivacyPreset(ctx, t, privacyStore, ownerProfile, "personal")
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := "nil-block-wiring-secret"
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb, func(s *UserGRPC) {
		// Keep show_online independently allowed; Blocks is deliberately nil.
		s.SocialGraph = alwaysFriendsGraph{}
	})
	_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{
		Status: "online", CustomStatus: &secret,
	})
	require.NoError(t, err)

	viewer := withUserAuthCtx(ctx, viewerAccount, viewerProfile)
	direct, err := cli.GetPresence(viewer, &userv1.GetPresenceRequest{ProfileId: ownerProfile.String()})
	require.NoError(t, err)
	assert.Nil(t, direct.GetPresenceStatus().CustomStatus, "foreign direct presence must fail closed when Blocks is nil")
	bulk, err := cli.GetBulkPresence(viewer, &userv1.GetBulkPresenceRequest{ProfileIds: []string{ownerProfile.String()}})
	require.NoError(t, err)
	assert.Nil(t, bulk.GetByProfileId()[ownerProfile.String()].CustomStatus, "foreign bulk presence must fail closed when Blocks is nil")
	assertCustomStatusPresentForDirectAndBulk(t, cli, withUserAuthCtx(ctx, ownerAccount, ownerProfile), ownerProfile, secret)
}

// TestCustomStatus_PresenceDependencyFailuresFailClosed is deliberately
// separate from generic audience tests: the Space and block S2S calls are
// privacy dependencies, and either unavailable dependency must never expose a
// target's session custom status through either read shape.
func TestCustomStatus_PresenceDependencyFailuresFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))

	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'dependencyowner', '7014', 'Owner', true),
       ($3, $4, 'dependencyviewer', '7015', 'Viewer', true)`,
		ownerProfile, ownerAccount, viewerProfile, viewerAccount)
	require.NoError(t, err)

	privacyStore := store.NewPrivacyStore(pool)
	spaceOnly := privacy.Audience{SpaceMembers: true, SpaceIDs: []string{uuid.NewString()}}
	spacePrivacy := privacy.SettingsForPreset("work")
	spacePrivacy.ShowOnline = spaceOnly
	_, err = privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(ownerProfile, spacePrivacy))
	require.NoError(t, err)
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	profiles := store.NewProfileStore(pool)
	secret := "dependency-secret"

	seed := startUserPrivacyTestServer(t, profiles, privacyStore, rdb)
	_, err = seed.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{
		Status: "online", CustomStatus: &secret,
	})
	require.NoError(t, err)
	viewer := withUserAuthCtx(ctx, viewerAccount, viewerProfile)

	t.Run("Space membership error omits custom status", func(t *testing.T) {
		cli := startUserPrivacyTestServer(t, profiles, privacyStore, rdb, func(s *UserGRPC) {
			s.SpaceCoMembership = customStatusFailingSpaceMembership{}
		})
		assertCustomStatusOmittedForDirectAndBulk(t, cli, viewer, ownerProfile)
	})

	t.Run("block lookup error omits custom status", func(t *testing.T) {
		// Keep the audience independently allowed: this assertion exercises the
		// Social block dependency rather than a generic audience denial.
		allowed := privacy.SettingsForPreset("personal")
		_, err := privacyStore.Upsert(ctx, store.PrivacyRowFromSettings(ownerProfile, allowed))
		require.NoError(t, err)
		cli := startUserPrivacyTestServer(t, profiles, privacyStore, rdb, func(s *UserGRPC) {
			s.SocialGraph = alwaysFriendsGraph{}
			s.Blocks = customStatusFailingBlocks{}
		})
		assertCustomStatusOmittedForDirectAndBulk(t, cli, viewer, ownerProfile)
	})
}

// TestCustomStatus_PresenceMalformedAndAmbiguousIdentityFailClosed proves that
// identity parsing itself is a privacy boundary.  The allowed control prevents
// malformed or duplicate metadata from passing merely because the fixture is
// otherwise denied by privacy settings.
func TestCustomStatus_PresenceMalformedAndAmbiguousIdentityFailClosed(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	integrationtest.ApplyUserDBMigrations(t, ctx, pool, repoRoot(t))

	ownerAccount, ownerProfile := uuid.New(), uuid.New()
	viewerAccount, viewerProfile := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'identityowner', '7016', 'Owner', true),
       ($3, $4, 'identityviewer', '7017', 'Viewer', true)`,
		ownerProfile, ownerAccount, viewerProfile, viewerAccount)
	require.NoError(t, err)

	privacyStore := store.NewPrivacyStore(pool)
	seedPrivacyPreset(ctx, t, privacyStore, ownerProfile, "personal")
	mr := miniredis.RunT(t)
	t.Cleanup(mr.Close)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	secret := "identity-secret"
	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), privacyStore, rdb, func(s *UserGRPC) {
		s.SocialGraph = alwaysFriendsGraph{}
		s.Blocks = stubProfileBlocks{}
	})
	_, err = cli.UpdatePresence(withProfileTier(ctx, ownerAccount, ownerProfile, "premium"), &userv1.UpdatePresenceRequest{
		Status: "online", CustomStatus: &secret,
	})
	require.NoError(t, err)

	assertCustomStatusPresentForDirectAndBulk(t, cli, withUserAuthCtx(ctx, viewerAccount, viewerProfile), ownerProfile, secret)
	for name, readCtx := range map[string]context.Context{
		"malformed profile metadata": metadata.AppendToOutgoingContext(ctx, authctx.HeaderProfileID, "not-a-uuid"),
		"ambiguous profile metadata": metadata.AppendToOutgoingContext(ctx,
			authctx.HeaderProfileID, ownerProfile.String(),
			authctx.HeaderProfileID, viewerProfile.String()),
	} {
		t.Run(name, func(t *testing.T) {
			assertCustomStatusOmittedForDirectAndBulk(t, cli, readCtx, ownerProfile)
		})
	}
}

type customStatusFailingSpaceMembership struct{}

func (customStatusFailingSpaceMembership) AreCoMembers(context.Context, uuid.UUID, uuid.UUID, []string) (bool, error) {
	return false, errors.New("space unavailable")
}

type customStatusFailingBlocks struct{}

func (customStatusFailingBlocks) AccountPairBlocked(context.Context, uuid.UUID, uuid.UUID) (bool, error) {
	return false, errors.New("blocks unavailable")
}

func assertCustomStatusPresentForDirectAndBulk(t *testing.T, cli userv1.UserServiceClient, ctx context.Context, profileID uuid.UUID, want string) {
	t.Helper()
	direct, err := cli.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: profileID.String()})
	require.NoError(t, err)
	require.Equal(t, want, direct.GetPresenceStatus().GetCustomStatus())
	bulk, err := cli.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{ProfileIds: []string{profileID.String()}})
	require.NoError(t, err)
	require.Equal(t, want, bulk.GetByProfileId()[profileID.String()].GetCustomStatus())
}

func assertCustomStatusOmittedForDirectAndBulk(t *testing.T, cli userv1.UserServiceClient, ctx context.Context, profileID uuid.UUID) {
	t.Helper()
	direct, err := cli.GetPresence(ctx, &userv1.GetPresenceRequest{ProfileId: profileID.String()})
	require.NoError(t, err)
	require.Nil(t, direct.GetPresenceStatus().CustomStatus)
	bulk, err := cli.GetBulkPresence(ctx, &userv1.GetBulkPresenceRequest{ProfileIds: []string{profileID.String()}})
	require.NoError(t, err)
	require.Nil(t, bulk.GetByProfileId()[profileID.String()].CustomStatus)
}

func withProfileTier(ctx context.Context, accountID, profileID uuid.UUID, tier string) context.Context {
	ctx = withUserAuthCtx(ctx, accountID, profileID)
	return metadata.AppendToOutgoingContext(ctx, authctx.HeaderSubscriptionTier, tier)
}

func stringPtr(s string) *string { return &s }
