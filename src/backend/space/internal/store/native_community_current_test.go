package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestNativeCommunityCurrent39ContinuousRefreshAndExpiredReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native community continuity")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "up")
	st := &SpaceStore{Pool: database.Pool}
	app, environment := uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment,
		OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "current39", TemplateID: "guild-default"})
	require.NoError(t, err)
	profile := uuid.New()
	in := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, SpaceID: space.ID, CorporationKey: "current39", OwnerGeneration: 1, SourceRevision: 1,
		SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{profile}}
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	var episode uuid.UUID
	var generation, floor int64
	var state string
	read := func() {
		t.Helper()
		require.NoError(t, database.Pool.QueryRow(ctx, `SELECT episode_id,generation,retired_through,state FROM space_community_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&episode, &generation, &floor, &state))
	}
	read()
	first := episode
	require.EqualValues(t, 1, generation)
	in.OperationID, in.SourceRevision = uuid.New(), 2
	in.SnapshotSHA256 = []byte("22222222222222222222222222222222")
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	read()
	require.Equal(t, first, episode, "continuous source refresh cannot manufacture a new episode")
	require.EqualValues(t, 1, generation)
	_, err = database.Pool.Exec(ctx, `UPDATE community_roster_members SET lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	_, err = database.Pool.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=clock_timestamp()-interval '1 second' WHERE space_id=$1`, space.ID)
	require.NoError(t, err)
	replay, err := st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.False(t, replay.Current)
	read()
	require.Equal(t, "RETIRED", state)
	require.Equal(t, generation, floor)
	in.OperationID, in.SourceRevision = uuid.New(), 3
	in.SnapshotSHA256 = []byte("33333333333333333333333333333333")
	in.LeaseExpiresAt = time.Now().Add(time.Minute)
	_, err = st.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	read()
	require.Equal(t, "ACTIVE", state)
	require.NotEqual(t, first, episode)
	require.Greater(t, generation, floor)
}

func TestNativeCommunityBootstrapCurrent39PublicationAndReplay(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL native community bootstrap")
	}
	ctx := context.Background()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "up")
	st := &SpaceStore{Pool: database.Pool}
	in := CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: uuid.New(), EnvironmentID: uuid.New(), OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "native-bootstrap", TemplateID: "guild-default"}
	space, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.True(t, created)
	read, err := st.ReadMemberAdmissionAuthority(ctx, &spacev1.GetMemberAdmissionAuthorityRequest{ProtocolVersion: 1, SpaceId: space.ID.String(), ProfileId: in.OwnerProfileID.String()})
	require.NoError(t, err)
	require.Equal(t, spacev1.MemberAdmissionPublicationKind_MEMBER_ADMISSION_PUBLICATION_KIND_NATIVE_COMMUNITY_OWNER_CREATE, read.GetOrdinary().GetPublicationKind())
	require.False(t, read.GetOrdinary().GetExplicitLegacy())
	require.Positive(t, read.GetOrdinary().GetHeadGeneration())
	require.Empty(t, read.GetOrdinary().GetRoleReceiptSha256())
	current, err := st.IsCommunityBootstrapPublicationCurrent(ctx, in.OperationID, space.ID)
	require.NoError(t, err)
	require.True(t, current)
	again, created, err := st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, space.ID, again.ID)
	require.NoError(t, st.RemoveMember(ctx, space.ID, in.OwnerProfileID))
	_, created, err = st.CreateCommunityBootstrapSpace(ctx, in)
	require.NoError(t, err)
	require.False(t, created)
	current, err = st.IsCommunityBootstrapPublicationCurrent(ctx, in.OperationID, space.ID)
	require.NoError(t, err)
	require.False(t, current, "historical bootstrap cannot recreate membership/Owner baseline")
}

func TestNativeCommunityRosterRechecksLeaseAfterFinalSQLWait(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL final roster SQL wait")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	database := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	database.Run(true, "up")
	st := &SpaceStore{Pool: database.Pool}
	app, environment := uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment,
		OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "late-roster", TemplateID: "guild-default"})
	require.NoError(t, err)
	blocker, err := database.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(context.Background())
	_, err = blocker.Exec(ctx, `LOCK TABLE community_roster_operations IN SHARE MODE`)
	require.NoError(t, err)
	lease := time.Now().Add(10 * time.Second)
	operation, profile := uuid.New(), uuid.New()
	done := make(chan error, 1)
	go func() {
		_, err := st.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: operation, ApplicationID: app, EnvironmentID: environment, SpaceID: space.ID, CorporationKey: "late-roster", OwnerGeneration: 1, SourceRevision: 1,
			SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: lease, ProfileIDs: []uuid.UUID{profile}})
		done <- err
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		err := database.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'INSERT INTO community_roster_operations%')`).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond, "writer must reach the actual final receipt INSERT wait")
	require.Eventually(t, func() bool {
		var expired bool
		err := database.Pool.QueryRow(ctx, `SELECT clock_timestamp()>$1`, lease).Scan(&expired)
		return err == nil && expired
	}, 15*time.Second, 10*time.Millisecond)
	require.NoError(t, blocker.Commit(ctx))
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrCommunityRosterConflict)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var rows int
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM community_roster_members WHERE space_id=$1`, space.ID).Scan(&rows))
	require.Zero(t, rows, "expired publication, head and receipt must all roll back")
	require.NoError(t, database.Pool.QueryRow(ctx, `SELECT count(*) FROM community_roster_operations WHERE operation_id=$1`, operation).Scan(&rows))
	require.Zero(t, rows)
}
