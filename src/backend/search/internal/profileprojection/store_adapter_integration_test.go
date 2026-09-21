package profileprojection

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	userv1 "voice.app/voice/user/v1"
	"voice/backend/pkg/integrationtest"
	"voice/backend/pkg/searchnormalization"
)

// TestApplyAndCheckpoint_CommitsProjectionAndOffsetTogether is the restart
// fence: a committed journal offset must never exist without its projection.
func TestApplyAndCheckpoint_CommitsProjectionAndOffsetTogether(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{
		"000003_space_lifecycle.up.sql",
		"000002_verification_type.up.sql",
		"000004_user_profile_projection.up.sql",
		"000005_user_profile_projection_snapshot.up.sql",
		"000006_user_profile_projection_quarantine.up.sql",
		"000007_user_profile_projection_fence.up.sql",
	} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}

	profileID, accountID := uuid.NewString(), uuid.NewString()
	event := &userv1.SearchProfileProjectionEvent{
		ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID, SourceRevision: 1, JournalOffset: 7,
		OccurredAt: timestamppb.Now(),
		Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{
			AccountId: accountID, Username: "atomic", Discriminator: "0001", DisplayName: "Atomic",
			UsernameSearchKey: "atomic", DisplayNameSearchKey: "atomic", NormalizationVersion: 1,
		}},
	}

	adapter := &StoreAdapter{Pool: pool}
	result, err := adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset())
	require.NoError(t, err)
	require.Equal(t, Applied, result)

	var checkpoint uint64
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_checkpoint WHERE singleton=true`).Scan(&checkpoint))
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM profile_search_documents WHERE profile_id=$1`, profileID).Scan(&revision))
	require.Equal(t, uint64(7), checkpoint)
	require.Equal(t, int64(1), revision)

	// Simulate JetStream redelivery after a process dies between the database
	// commit and Ack: the inbox fence keeps one document and the checkpoint.
	_, err = adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset())
	require.NoError(t, err)
	var inboxCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_user_profile_inbox WHERE event_id=$1`, event.GetEventId()).Scan(&inboxCount))
	require.Equal(t, 1, inboxCount)
	require.NoError(t, pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_checkpoint WHERE singleton=true`).Scan(&checkpoint))
	require.Equal(t, uint64(7), checkpoint)

	conflict := proto.Clone(event).(*userv1.SearchProfileProjectionEvent)
	conflict.EventId = uuid.NewString()
	conflict.GetUpsert().DisplayName = "Conflicting same revision"
	conflict.GetUpsert().DisplayNameSearchKey = searchnormalization.V1.Normalize(conflict.GetUpsert().GetDisplayName())
	_, err = adapter.ApplyAndCheckpoint(ctx, conflict, 7)
	require.Error(t, err)
	var quarantinedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT quarantined_at FROM search_user_profile_inbox WHERE event_id=$1`, conflict.GetEventId()).Scan(&quarantinedAt))
	require.NotNil(t, quarantinedAt)
	var fenceRevision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_fence WHERE profile_id=$1`, profileID).Scan(&fenceRevision))
	require.Equal(t, int64(1), fenceRevision)
	// Both the accepted event and the conflicting one can be redelivered without
	// moving the durable authority fence away from the accepted payload.
	_, err = adapter.ApplyAndCheckpoint(ctx, event, 7)
	require.NoError(t, err)
	_, err = adapter.ApplyAndCheckpoint(ctx, conflict, 7)
	require.Error(t, err)
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_fence WHERE profile_id=$1`, profileID).Scan(&fenceRevision))
	require.Equal(t, int64(1), fenceRevision)
	// The rollback migration removes quarantine duplicates before restoring the
	// historical unconditional profile/revision index.
	integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000006_user_profile_projection_quarantine.down.sql"))
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_user_profile_inbox WHERE profile_id=$1 AND source_revision=1`, profileID).Scan(&remaining))
	require.Equal(t, 1, remaining)
}

// TestFence_InverseDeliveryOrderRetainsNewerRevision forces N+1 to commit
// before N. The per-profile fence rejects the stale N after N+1 is durable.
func TestFence_InverseDeliveryOrderRetainsNewerRevision(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_fence", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	profileID, accountID := uuid.NewString(), uuid.NewString()
	makeEvent := func(revision, offset uint64, name string) *userv1.SearchProfileProjectionEvent {
		return &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID, SourceRevision: revision, JournalOffset: offset, OccurredAt: timestamppb.Now(), Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: accountID, Username: "fence", Discriminator: "0001", DisplayName: name, UsernameSearchKey: "fence", DisplayNameSearchKey: searchnormalization.V1.Normalize(name), NormalizationVersion: 1}}}
	}
	newer, older := makeEvent(2, 2, "N+1"), makeEvent(1, 1, "N")
	newerLocked, releaseNewer := make(chan struct{}), make(chan struct{})
	olderAttempted := make(chan struct{})
	adapter := &StoreAdapter{Pool: pool, BeforeFenceLock: func(event *userv1.SearchProfileProjectionEvent) {
		if event.GetEventId() == older.GetEventId() {
			close(olderAttempted)
		}
	}, AfterFenceLock: func(event *userv1.SearchProfileProjectionEvent) {
		if event.GetEventId() == newer.GetEventId() {
			close(newerLocked)
			<-releaseNewer
		}
	}}
	newerResult := make(chan error, 1)
	olderResult := make(chan error, 1)
	go func() { _, err := adapter.ApplyAndCheckpoint(ctx, newer, 2); newerResult <- err }()
	<-newerLocked // N+1 owns the durable fence; N is now forced to wait behind it.
	go func() { _, err := adapter.ApplyAndCheckpoint(ctx, older, 1); olderResult <- err }()
	<-olderAttempted
	select {
	case err := <-olderResult:
		t.Fatalf("older N completed before N+1 released its fence: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseNewer)
	require.NoError(t, <-newerResult)
	require.NoError(t, <-olderResult)
	var revision int64
	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision,display_name FROM profile_search_documents WHERE profile_id=$1`, profileID).Scan(&revision, &name))
	require.Equal(t, int64(2), revision)
	require.Equal(t, "N+1", name)
}

// TestSnapshotState_PersistsRestartCursor proves a crashed bootstrap resumes
// the same H-bound snapshot rather than reading live authority rows anew.
func TestSnapshotState_PersistsRestartCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_snapshot", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	adapter := &StoreAdapter{Pool: pool}
	require.NoError(t, adapter.StartSnapshot(ctx, 41))
	require.NoError(t, adapter.AdvanceSnapshotCursor(ctx, "signed-cursor"))
	state, err := adapter.SnapshotState(ctx)
	require.NoError(t, err)
	require.Equal(t, "snapshot", state.Phase)
	require.Equal(t, uint64(41), state.HighWatermark)
	require.Equal(t, "signed-cursor", state.Cursor)
	require.NoError(t, adapter.FinishSnapshot(ctx))
	state, err = adapter.SnapshotState(ctx)
	require.NoError(t, err)
	require.Equal(t, "replay", state.Phase)
	require.Empty(t, state.Cursor)

	// An expired opaque cursor is reset before a new Begin call; stale H/cursor
	// state must not survive and poison the replacement snapshot session.
	require.NoError(t, adapter.StartSnapshot(ctx, 99))
	require.NoError(t, adapter.AdvanceSnapshotCursor(ctx, "expired-signed-cursor"))
	require.NoError(t, adapter.ResetSnapshot(ctx))
	state, err = adapter.SnapshotState(ctx)
	require.NoError(t, err)
	require.Equal(t, "idle", state.Phase)
	require.Zero(t, state.HighWatermark)
	require.Empty(t, state.Cursor)
}

func searchProjectionRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}
