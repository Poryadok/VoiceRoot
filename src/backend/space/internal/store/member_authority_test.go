package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Before the implementation exists this still reaches PostgreSQL: the missing
// durable head is the intended RED, rather than a compile or missing-file error.
func applyMemberAuthorityIfPresent(t *testing.T, st *SpaceStore) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "src/backend/migrations/space_db/000033_membership_authority.up.sql"))
	if os.IsNotExist(err) {
		return
	}
	require.NoError(t, err)
	_, err = st.Pool.Exec(context.Background(), string(raw))
	require.NoError(t, err)
}

func TestMemberAuthorityLegacyIdentityAndRemovalFence(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	owner, profile := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, owner, "Membership authority", "", "public")
	require.NoError(t, err)
	_, err = st.JoinSpace(ctx, space.ID, profile, uuid.New())
	require.NoError(t, err)
	applyMemberAuthorityIfPresent(t, st)
	var generation, retired int64
	var publication uuid.UUID
	var state string
	var legacy bool
	read := func() {
		t.Helper()
		require.NoError(t, pool.QueryRow(ctx, `SELECT generation,retired_through,publication_id,state,legacy
		 FROM space_ordinary_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).
			Scan(&generation, &retired, &publication, &state, &legacy))
	}
	read()
	require.Zero(t, generation)
	require.True(t, legacy)
	require.Equal(t, "ACTIVE", state)
	require.NotEqual(t, uuid.Nil, publication)
	original := publication
	read()
	require.Equal(t, original, publication, "reads cannot manufacture a new legacy publication")
	require.NoError(t, st.RemoveMember(ctx, space.ID, profile))
	read()
	require.Equal(t, "RETIRED", state)
	require.Positive(t, generation)
	require.Equal(t, generation, retired)
	_, err = pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, profile)
	require.Error(t, err, "a raw late membership writer cannot resurrect the retired incarnation")
	read()
	require.Equal(t, "RETIRED", state)
}

func TestMemberAuthorityCommunityContinuityAndExpiredReplay(t *testing.T) {
	ctx := context.Background()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyMemberAuthorityIfPresent(t, st)
	app, env, profile := uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{
		OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env,
		OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "episode", TemplateID: "guild-default",
	})
	require.NoError(t, err)
	in := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env,
		CorporationKey: "episode", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1,
		SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{profile}}
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	var generation, floor int64
	var episode uuid.UUID
	var state string
	read := func() {
		t.Helper()
		require.NoError(t, pool.QueryRow(ctx, `SELECT generation,retired_through,episode_id,state
		 FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).
			Scan(&generation, &floor, &episode, &state))
	}
	read()
	require.EqualValues(t, 1, generation)
	require.Equal(t, "ACTIVE", state)
	first := episode
	in.OperationID, in.SourceRevision = uuid.New(), 2
	in.SnapshotSHA256 = []byte("22222222222222222222222222222222")
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	read()
	require.Equal(t, first, episode, "continuous refresh must preserve the eligibility episode")
	require.EqualValues(t, 1, generation)
	_, err = pool.Exec(ctx, `UPDATE community_roster_members SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	historical, err := st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	require.True(t, historical.Replayed)
	require.False(t, historical.Current, "exact replay is historical and cannot renew an expired source")
	read()
	require.Equal(t, "RETIRED", state)
	require.Equal(t, first, episode)
	in.OperationID, in.SourceRevision, in.LeaseExpiresAt = uuid.New(), 3, time.Now().Add(time.Minute)
	in.SnapshotSHA256 = []byte("33333333333333333333333333333333")
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	read()
	require.NotEqual(t, first, episode)
	require.Greater(t, generation, int64(1))
	require.GreaterOrEqual(t, floor, int64(1))
}

func TestMemberAuthorityRosterWaitRechecksExpiredEpisode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool := startSpacePostgresForStoreTest(t, ctx)
	applySpaceMigrationForStoreTest(t, ctx, pool)
	st := &SpaceStore{Pool: pool}
	applyMemberAuthorityIfPresent(t, st)
	app, env, profile := uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{
		OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env,
		OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "held-episode", TemplateID: "guild-default",
	})
	require.NoError(t, err)
	in := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: env,
		CorporationKey: "held-episode", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1,
		SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{profile}}
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	var before uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT episode_id FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&before))
	barrier, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = barrier.Rollback(context.Background()) }()
	_, err = barrier.Exec(ctx, `SELECT pg_advisory_xact_lock($1::bigint)`, spaceMutationAdvisoryKey(space.ID))
	require.NoError(t, err)
	workerPool := r22SecondSpacePool(t, ctx, pool, "member_authority_roster_wait")
	in.OperationID, in.SourceRevision = uuid.New(), 2
	in.SnapshotSHA256 = []byte("22222222222222222222222222222222")
	result := make(chan error, 1)
	go func() {
		_, callErr := (&SpaceStore{Pool: workerPool}).ApplyCommunityRoster(ctx, in)
		result <- callErr
	}()
	waitR20NamedPoolLock(t, ctx, pool, "member_authority_roster_wait")
	_, err = barrier.Exec(ctx, `UPDATE community_roster_members SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	_, err = barrier.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	require.NoError(t, barrier.Commit(ctx))
	require.NoError(t, <-result)
	var after uuid.UUID
	var generation int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT episode_id,generation FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&after, &generation))
	require.NotEqual(t, before, after, "expiry while blocked must end the prior eligibility episode")
	require.EqualValues(t, 2, generation)
}
