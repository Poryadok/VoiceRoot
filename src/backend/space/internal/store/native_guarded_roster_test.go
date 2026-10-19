package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"voice/backend/pkg/integrationtest"
)

func TestNative44RosterWriterRefreshReplayAndExpiry(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	st := &SpaceStore{Pool: db.Pool}
	app, environment := uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "guarded44", TemplateID: "guild-default"})
	require.NoError(t, err)
	db.Run(true, "goto", "44")
	require.NoError(t, st.InitializeNativeMembership(ctx, space.ID))
	serving := &SpaceStore{Pool: nativeReadinessLogin(t, ctx, db.Pool)}
	profile := uuid.New()
	in := CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, CorporationKey: "guarded44", SpaceID: space.ID, OwnerGeneration: 1, SourceRevision: 1, SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: time.Now().Add(time.Minute), ProfileIDs: []uuid.UUID{profile}}
	_, err = serving.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	var first, next uuid.UUID
	var state, boundary string
	var revision, replayRevision int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT episode_id,boundary_state FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&first, &boundary))
	require.Equal(t, "KNOWN", boundary)
	in.OperationID, in.SourceRevision = uuid.New(), 2
	in.SnapshotSHA256 = []byte("22222222222222222222222222222222")
	in.LeaseExpiresAt = time.Now().Add(8 * time.Second)
	_, err = serving.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT episode_id,ledger_revision FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&next, &revision))
	require.Equal(t, first, next, "continuous renewal keeps the actual episode")
	replay, err := serving.ApplyCommunityRoster(ctx, in)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT ledger_revision FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&replayRevision))
	require.Equal(t, revision, replayRevision, "completed replay never runs new history callbacks")
	_, err = db.Pool.Exec(ctx, `SELECT pg_sleep(GREATEST(0,EXTRACT(epoch FROM ($1::timestamptz-clock_timestamp())))+0.05)`, in.LeaseExpiresAt)
	require.NoError(t, err)
	_, err = serving.RevokeExpiredCommunityRosterMemberships(ctx)
	require.NoError(t, err)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT state FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, profile).Scan(&state))
	require.Equal(t, "RETIRED", state)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT member_count FROM public.spaces WHERE id=$1`, space.ID).Scan(&count))
	require.Equal(t, 1, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count)

	// A real wait at the final operation receipt must not commit any new
	// source/history after the44 frame deadline, even with a live new lease.
	blocker, err := db.Pool.Begin(ctx)
	require.NoError(t, err)
	defer blocker.Rollback(ctx)
	_, err = blocker.Exec(ctx, `LOCK TABLE public.community_roster_operations IN SHARE MODE`)
	require.NoError(t, err)
	lateProfile := uuid.New()
	late := in
	late.OperationID, late.SourceRevision = uuid.New(), 3
	late.SnapshotSHA256 = []byte("33333333333333333333333333333333")
	late.ProfileIDs = []uuid.UUID{lateProfile}
	late.LeaseExpiresAt = time.Now().Add(time.Minute)
	done := make(chan error, 1)
	go func() { _, err := serving.ApplyCommunityRoster(ctx, late); done <- err }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE wait_event_type='Lock' AND query LIKE 'INSERT INTO community_roster_operations%')`).Scan(&waiting)
		return err == nil && waiting
	}, 10*time.Second, 10*time.Millisecond)
	_, err = db.Pool.Exec(ctx, `SELECT pg_sleep(5.05)`)
	require.NoError(t, err)
	require.NoError(t, blocker.Commit(ctx))
	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("bounded held receipt writer did not terminate")
	}
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, lateProfile).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM public.community_roster_operations WHERE operation_id=$1`, late.OperationID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM native_membership_guard.frames WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count)
}
