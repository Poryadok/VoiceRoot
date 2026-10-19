package store

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
	spacev1 "voice.app/voice/space/v1"
	"voice/backend/pkg/integrationtest"
)

func TestEffectiveReportReadUsesRealCreatorEpisode(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	st := &SpaceStore{Pool: db.Pool}
	create, account, viewer := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "Protected history", Visibility: "private"})
	space, err := create()
	require.NoError(t, err)
	chat := uuid.New()
	node, err := st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chat})
	require.NoError(t, err)
	cut, err := st.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.NoError(t, err)
	require.True(t, cut.Eligible)
	require.True(t, cut.HistoryKnown)
	require.False(t, cut.HistoryVisibleSince.IsZero())
	require.Equal(t, node.ID, cut.TreeNodeID)
	require.NotEqual(t, uuid.Nil, cut.EpisodeID)
	require.Positive(t, cut.EpisodeGeneration)
	require.Positive(t, cut.LedgerRevision)
	require.Positive(t, cut.SpaceRevision)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, uuid.New(), chat)
	require.ErrorIs(t, err, ErrSpaceReportReadDenied)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, viewer, uuid.New())
	require.ErrorIs(t, err, ErrSpaceReportReadDenied)
	foreignCreate, _, foreignViewer := signedNativeCreatorCurrent(t, st, &spacev1.CreateSpaceRequest{Name: "Foreign scope", Visibility: "private"})
	foreignSpace, err := foreignCreate()
	require.NoError(t, err)
	foreignChat := uuid.New()
	_, err = st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: foreignSpace.ID, Kind: TreeKindTextChat, ChatID: &foreignChat})
	require.NoError(t, err)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, viewer, foreignChat)
	require.ErrorIs(t, err, ErrSpaceReportReadDenied)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, foreignViewer, chat)
	require.ErrorIs(t, err, ErrSpaceReportReadDenied)
	old, err := db.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer old.Rollback(ctx)
	var count int
	require.NoError(t, old.QueryRow(ctx, `SELECT count(*) FROM space_members`).Scan(&count))
	stale := *st
	stale.tx = old
	_, err = stale.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable)
	require.NoError(t, old.Rollback(ctx))

	t.Run("final clock is after actual head row wait", func(t *testing.T) {
		raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()
		holder, err := db.Pool.Begin(raceCtx)
		require.NoError(t, err)
		defer holder.Rollback(ctx)
		var holderPID int32
		require.NoError(t, holder.QueryRow(raceCtx, `SELECT pg_backend_pid()`).Scan(&holderPID))
		var revision int64
		require.NoError(t, holder.QueryRow(raceCtx, `SELECT ledger_revision FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2 FOR UPDATE`, space.ID, viewer).Scan(&revision))
		done := make(chan error, 1)
		go func() {
			result, err := st.ReadSpaceReportReadContext(raceCtx, space.ID, viewer, chat)
			if result != nil {
				done <- errors.New("expired read returned positioning facts")
				return
			}
			done <- err
		}()
		var blocked bool
		for i := 0; i < 200; i++ {
			require.NoError(t, db.Pool.QueryRow(raceCtx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity a WHERE a.datname=current_database() AND $1=ANY(pg_blocking_pids(a.pid)))`, holderPID).Scan(&blocked))
			if blocked {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		require.True(t, blocked, "reader must reach actual held row lock")
		_, err = holder.Exec(raceCtx, `SELECT pg_sleep(5.05)`)
		require.NoError(t, err)
		require.NoError(t, holder.Commit(raceCtx))
		require.ErrorIs(t, <-done, ErrEffectiveEpisodeUnavailable)
		var after int64
		require.NoError(t, db.Pool.QueryRow(ctx, `SELECT ledger_revision FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, viewer).Scan(&after))
		require.Equal(t, revision, after, "read cannot repair or advance a ledger")
	})
	request := lifecycleScheduleRequest(space.ID, uuid.New())
	request.ConfirmationName = space.Name
	_, err = st.ReserveLifecycleSchedule(ctx, account, viewer, 7, request)
	require.NoError(t, err)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "actual schedule reservation freezes disclosure")
}

func TestEffectiveReportReadUsesRealFiniteCommunityPublication(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "41")
	st := &SpaceStore{Pool: db.Pool}
	app, environment, viewer, chat := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	space, _, err := st.CreateCommunityBootstrapSpace(ctx, CommunityBootstrapSpaceInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, OwnerAccountID: uuid.New(), OwnerProfileID: uuid.New(), CorporationKey: "report-context", TemplateID: "guild-default"})
	require.NoError(t, err)
	_, err = st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chat})
	require.NoError(t, err)
	var lease time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '5 seconds'`).Scan(&lease))
	// Exercise the real producer ABI around the real native publication. This
	// explicit same-tx test composition does not claim every production roster
	// writer has been wired or that a global cutover barrier exists.
	require.NoError(t, st.withOwnershipScope(ctx, []uuid.UUID{space.ID}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureEffectiveMemberMutations(ctx, space.ID, []uuid.UUID{viewer})
		if err != nil {
			return err
		}
		_, err = scoped.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, SpaceID: space.ID, CorporationKey: "report-context", OwnerGeneration: 1, SourceRevision: 1, SnapshotSHA256: []byte("11111111111111111111111111111111"), LeaseExpiresAt: lease, ProfileIDs: []uuid.UUID{viewer}})
		if err != nil {
			return err
		}
		return scoped.finishEffectiveMemberMutations(ctx, capture)
	}))
	cut, err := st.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.NoError(t, err)
	require.True(t, cut.Eligible)
	require.True(t, cut.HistoryKnown)
	require.NotNil(t, cut.SourceBinding.GetCommunity())
	require.Positive(t, cut.ValidityRemaining)
	require.LessOrEqual(t, cut.ValidityRemaining, 5*time.Second)
	// The current production roster method has not yet been wrapped by the
	// native owner. A real renewal cannot be adopted by this reader as history.
	var renewedLease time.Time
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT clock_timestamp()+interval '8 seconds'`).Scan(&renewedLease))
	_, err = st.ApplyCommunityRoster(ctx, CommunityRosterInput{OperationID: uuid.New(), ApplicationID: app, EnvironmentID: environment, SpaceID: space.ID, CorporationKey: "report-context", OwnerGeneration: 1, SourceRevision: 2, SnapshotSHA256: []byte("22222222222222222222222222222222"), LeaseExpiresAt: renewedLease, ProfileIDs: []uuid.UUID{viewer}})
	require.NoError(t, err)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable, "unjournaled actual source renewal is not a reader cutover")
	var revision int64
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT ledger_revision FROM space_effective_member_heads WHERE space_id=$1 AND profile_id=$2`, space.ID, viewer).Scan(&revision))
	require.Equal(t, cut.LedgerRevision, revision, "reader must not update stale source witnesses")
	require.Eventually(t, func() bool {
		var expired bool
		return db.Pool.QueryRow(ctx, `SELECT clock_timestamp()>=$1`, renewedLease).Scan(&expired) == nil && expired
	}, 10*time.Second, 10*time.Millisecond)
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrSpaceReportReadDenied, "strict lease expiry cannot disclose saved history")
}

func TestEffectiveReportReadDoesNotInventLegacyCutover(t *testing.T) {
	ctx := context.Background()
	db := integrationtest.NewSourceMigrationFixture(t, ctx, filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	db.Run(true, "goto", "34")
	st := &SpaceStore{Pool: db.Pool}
	viewer, chat := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, viewer, "Uncut legacy", "", "public")
	require.NoError(t, err)
	_, err = st.UpsertTreeNode(ctx, UpsertTreeNodeInput{SpaceID: space.ID, Kind: TreeKindTextChat, ChatID: &chat})
	require.NoError(t, err)
	db.Run(true, "goto", "41")
	_, err = st.ReadSpaceReportReadContext(ctx, space.ID, viewer, chat)
	require.ErrorIs(t, err, ErrEffectiveEpisodeUnavailable)
	var count int
	require.NoError(t, db.Pool.QueryRow(ctx, `SELECT count(*) FROM space_effective_member_heads WHERE space_id=$1`, space.ID).Scan(&count))
	require.Zero(t, count, "read cannot invent cutover")
}
