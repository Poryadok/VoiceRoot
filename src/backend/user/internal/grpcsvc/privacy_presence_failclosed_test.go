package grpcsvc

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/integrationtest"
	"voice/backend/user/internal/store"

	userv1 "voice.app/voice/user/v1"
)

// TestGetPresence_NoPrivacyRow_FailClosed documents missing privacy_settings denies online visibility.
func TestGetPresence_NoPrivacyRow_FailClosed(t *testing.T) {
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

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb)

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

// TestGetPresence_WorkPreset_SpaceMembersOnly documents work preset show_online = space members.
func TestGetPresence_WorkPreset_SpaceMembersOnly(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "userdb", "")
	applyUserPrivacyMigrations(t, ctx, pool)

	ownerAccount := uuid.New()
	ownerProfile := uuid.New()
	coworkerAccount := uuid.New()
	coworkerProfile := uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO profiles (id, account_id, username, discriminator, display_name, is_primary)
VALUES ($1, $2, 'owner', '4444', 'Owner', true),
       ($3, $4, 'coworker', '5555', 'Coworker', true)`,
		ownerProfile, ownerAccount, coworkerProfile, coworkerAccount)
	require.NoError(t, err)
	seedPrivacyPreset(ctx, t, store.NewPrivacyStore(pool), ownerProfile, "work")

	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	cli := startUserPrivacyTestServer(t, store.NewProfileStore(pool), store.NewPrivacyStore(pool), rdb,
		func(s *UserGRPC) {
			s.SpaceCoMembership = stubSpaceCoMembership{co: map[string]bool{
				privacyPairKey(coworkerProfile, ownerProfile): true,
			}}
		},
	)

	_, err = cli.UpdatePresence(withUserAuthCtx(ctx, ownerAccount, ownerProfile), &userv1.UpdatePresenceRequest{
		Status: "online",
	})
	require.NoError(t, err)

	visible, err := cli.GetPresence(withUserAuthCtx(ctx, coworkerAccount, coworkerProfile), &userv1.GetPresenceRequest{
		ProfileId: ownerProfile.String(),
	})
	require.NoError(t, err)
	require.Equal(t, "online", visible.GetPresenceStatus().GetStatus())

	hidden, err := cli.GetPresence(withUserAuthCtx(ctx, uuid.New(), uuid.New()), &userv1.GetPresenceRequest{
		ProfileId: ownerProfile.String(),
	})
	require.NoError(t, err)
	require.Empty(t, hidden.GetPresenceStatus().GetStatus())
}

// TestGetPrivacySettings_WorkPreset documents work preset from privacy.md.
func TestGetPrivacySettings_WorkPreset(t *testing.T) {
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
VALUES ($1, $2, 'work', '9999', 'Work', true)`,
		profileID, accountID)
	require.NoError(t, err)
	seedPrivacyPreset(ctx, t, store.NewPrivacyStore(pool), profileID, "work")

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
	require.True(t, ps.GetShowOnline().GetSpaceMembers())
	require.False(t, ps.GetShowOnline().GetFriends())
	require.True(t, ps.GetAllowDm().GetSpaceMembers())
	require.True(t, ps.GetAllowDm().GetFriends())
}
