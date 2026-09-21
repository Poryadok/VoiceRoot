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
		"000008_user_profile_projection_generations.up.sql",
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
	require.NoError(t, pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=1`).Scan(&checkpoint))
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_generation_documents WHERE generation=1 AND profile_id=$1`, profileID).Scan(&revision))
	require.Equal(t, uint64(7), checkpoint)
	require.Equal(t, int64(1), revision)

	// Simulate JetStream redelivery after a process dies between the database
	// commit and Ack: the inbox fence keeps one document and the checkpoint.
	_, err = adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset())
	require.NoError(t, err)
	var inboxCount int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_user_profile_generation_inbox WHERE generation=1 AND event_id=$1`, event.GetEventId()).Scan(&inboxCount))
	require.Equal(t, 1, inboxCount)
	require.NoError(t, pool.QueryRow(ctx, `SELECT journal_offset FROM search_user_profile_generation_checkpoint WHERE generation=1`).Scan(&checkpoint))
	require.Equal(t, uint64(7), checkpoint)

	conflict := proto.Clone(event).(*userv1.SearchProfileProjectionEvent)
	conflict.EventId = uuid.NewString()
	conflict.JournalOffset = 8
	conflict.GetUpsert().DisplayName = "Conflicting same revision"
	conflict.GetUpsert().DisplayNameSearchKey = searchnormalization.V1.Normalize(conflict.GetUpsert().GetDisplayName())
	_, err = adapter.ApplyAndCheckpoint(ctx, conflict, conflict.GetJournalOffset())
	require.Error(t, err)
	var quarantinedAt *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT quarantined_at FROM search_user_profile_generation_inbox WHERE generation=1 AND event_id=$1`, conflict.GetEventId()).Scan(&quarantinedAt))
	require.NotNil(t, quarantinedAt)
	var fenceRevision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_generation_fence WHERE generation=1 AND profile_id=$1`, profileID).Scan(&fenceRevision))
	require.Equal(t, int64(1), fenceRevision)
	// Both the accepted event and the conflicting one can be redelivered without
	// moving the durable authority fence away from the accepted payload.
	_, err = adapter.ApplyAndCheckpoint(ctx, event, 7)
	require.NoError(t, err)
	result, err = adapter.ApplyAndCheckpoint(ctx, conflict, conflict.GetJournalOffset())
	require.NoError(t, err)
	require.Equal(t, NoopDuplicate, result)
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_generation_fence WHERE generation=1 AND profile_id=$1`, profileID).Scan(&fenceRevision))
	require.Equal(t, int64(1), fenceRevision)
	malformed := proto.Clone(event).(*userv1.SearchProfileProjectionEvent)
	malformed.GetUpsert().UsernameSearchKey = ""
	_, err = adapter.ApplyAndCheckpoint(ctx, malformed, malformed.GetJournalOffset())
	require.Error(t, err)
	var malformedQuarantined *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT quarantined_at FROM search_user_profile_generation_inbox WHERE generation=1 AND event_id=$1`, event.GetEventId()).Scan(&malformedQuarantined))
	require.NotNil(t, malformedQuarantined)
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_generation_fence WHERE generation=1 AND profile_id=$1`, profileID).Scan(&fenceRevision))
	require.Equal(t, int64(1), fenceRevision)
}

// TestFence_InverseDeliveryOrderRetainsNewerRevision forces N+1 to commit
// before N. The per-profile fence rejects the stale N after N+1 is durable.
func TestFence_InverseDeliveryOrderRetainsNewerRevision(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_fence", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql", "000008_user_profile_projection_generations.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	profileID, accountID := uuid.NewString(), uuid.NewString()
	makeEvent := func(revision, offset uint64, name string) *userv1.SearchProfileProjectionEvent {
		return &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID, SourceRevision: revision, JournalOffset: offset, OccurredAt: timestamppb.Now(), Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: accountID, Username: "fence", Discriminator: "0001", DisplayName: name, UsernameSearchKey: "fence", DisplayNameSearchKey: searchnormalization.V1.Normalize(name), NormalizationVersion: 1}}}
	}
	newer, older := makeEvent(2, 1, "N+1"), makeEvent(1, 2, "N")
	newerLocked, releaseNewer := make(chan struct{}), make(chan struct{})
	olderAttempted := make(chan struct{})
	adapter := &StoreAdapter{Pool: pool, AfterFenceLock: func(event *userv1.SearchProfileProjectionEvent) {
		if event.GetEventId() == newer.GetEventId() {
			close(newerLocked)
			<-releaseNewer
		}
	}}
	newerResult := make(chan error, 1)
	type applyOutcome struct {
		result ApplyResult
		err    error
	}
	olderResult := make(chan applyOutcome, 1)
	go func() { _, err := adapter.ApplyAndCheckpoint(ctx, newer, 1); newerResult <- err }()
	<-newerLocked // N+1 owns the durable fence; N is now forced to wait behind it.
	go func() {
		// Generation-scoped checkpoint evidence is locked before the profile
		// fence, so a late hook cannot prove this delivery has started.
		close(olderAttempted)
		result, err := adapter.ApplyAndCheckpoint(ctx, older, 2)
		olderResult <- applyOutcome{result: result, err: err}
	}()
	<-olderAttempted
	select {
	case outcome := <-olderResult:
		t.Fatalf("older N completed before N+1 released its fence: %+v", outcome)
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseNewer)
	require.NoError(t, <-newerResult)
	olderOutcome := <-olderResult
	require.NoError(t, olderOutcome.err)
	require.Equal(t, NoopStale, olderOutcome.result)
	var revision int64
	var name string
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision,display_name FROM search_user_profile_generation_documents WHERE generation=1 AND profile_id=$1`, profileID).Scan(&revision, &name))
	require.Equal(t, int64(2), revision)
	require.Equal(t, "N+1", name)
	var fenceRevision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT source_revision FROM search_user_profile_generation_fence WHERE generation=1 AND profile_id=$1`, profileID).Scan(&fenceRevision))
	require.Equal(t, int64(2), fenceRevision)
}

// TestSnapshotState_PersistsRestartCursor proves a crashed bootstrap resumes
// the same H-bound snapshot rather than reading live authority rows anew.
func TestSnapshotState_PersistsRestartCursor(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_snapshot", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql", "000008_user_profile_projection_generations.up.sql"} {
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

func TestGenerationRoute_PromotesAndRollsBackWithoutLegacyFallback(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_generation", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql", "000008_user_profile_projection_generations.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}

	lease, acquired, err := TryStartGeneration(ctx, pool, 2)
	require.NoError(t, err)
	require.True(t, acquired)
	defer FinishGenerationRebuild(ctx, lease)
	profileID, accountID := uuid.NewString(), uuid.NewString()
	event := &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID, SourceRevision: 1, JournalOffset: 1, Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: accountID, Username: "route", Discriminator: "0001", DisplayName: "Route", UsernameSearchKey: "route", DisplayNameSearchKey: "route", NormalizationVersion: 1}}}
	adapter := &StoreAdapter{Pool: pool, Generation: 2}
	require.NoError(t, adapter.StartSnapshot(ctx, 1))
	_, err = adapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset())
	require.NoError(t, err)
	require.NoError(t, adapter.FinishSnapshot(ctx))
	activeAdapter := &StoreAdapter{Pool: pool, Generation: 1}
	_, err = activeAdapter.ApplyAndCheckpoint(ctx, event, event.GetJournalOffset())
	require.NoError(t, err)
	require.NoError(t, MarkGenerationReady(ctx, pool, 2))
	route, err := PromoteGeneration(ctx, pool, 2)
	require.NoError(t, err)
	require.Equal(t, uint64(2), route.Active)
	require.Equal(t, uint64(1), route.Rollback)

	route, err = RollbackGeneration(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, uint64(1), route.Active)
	require.Equal(t, uint64(2), route.Rollback)

	// A broken route is an availability failure, never a read from unscoped data.
	_, err = pool.Exec(ctx, `DELETE FROM search_user_profile_generation_route WHERE singleton=true`)
	require.NoError(t, err)
	_, err = LoadGenerationRoute(ctx, pool)
	require.Error(t, err)
}

func TestGenerationMigration_Preserves428LegacyConflictTargets(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_generation_compat", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	profileID, eventID := uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `INSERT INTO search_user_profile_inbox(event_id,profile_id,source_revision,payload_sha256) VALUES($1,$2,1,decode(repeat('00',32),'hex'))`, eventID, profileID)
	require.NoError(t, err)
	integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000008_user_profile_projection_generations.up.sql"))
	// A #428 binary retains its original table and ON CONFLICT(event_id) target.
	_, err = pool.Exec(ctx, `INSERT INTO search_user_profile_inbox(event_id,profile_id,source_revision,payload_sha256) VALUES($1,$2,1,decode(repeat('00',32),'hex')) ON CONFLICT(event_id) DO NOTHING`, eventID, profileID)
	require.NoError(t, err)
	var copied int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_user_profile_generation_inbox WHERE generation=1 AND event_id=$1`, eventID).Scan(&copied))
	require.Equal(t, 1, copied)
}

func TestLegacyQuarantineDownMigration_RemovesOnlyQuarantinedConflict(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_quarantine_down", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	profileID, acceptedEventID, quarantinedEventID := uuid.New(), uuid.New(), uuid.New()
	_, err := pool.Exec(ctx, `
INSERT INTO search_user_profile_inbox(event_id,profile_id,source_revision,payload_sha256,quarantined_at,quarantine_reason)
VALUES
  ($1,$3,1,decode(repeat('00',32),'hex'),NULL,NULL),
  ($2,$3,1,decode(repeat('11',32),'hex'),now(),'conflicting same revision')`, acceptedEventID, quarantinedEventID, profileID)
	require.NoError(t, err)

	integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", "000006_user_profile_projection_quarantine.down.sql"))
	var retainedEventID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT event_id FROM search_user_profile_inbox WHERE profile_id=$1 AND source_revision=1`, profileID).Scan(&retainedEventID))
	require.Equal(t, acceptedEventID, retainedEventID)
	var remaining int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_user_profile_inbox WHERE profile_id=$1 AND source_revision=1`, profileID).Scan(&remaining))
	require.Equal(t, 1, remaining)
}

func TestGenerationRoute_PromotionWaitsForRouteLock(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_generation_route_lock", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql", "000008_user_profile_projection_generations.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	_, err := pool.Exec(ctx, `INSERT INTO search_user_profile_generations(generation,state,ready_at,evidence_sha256) VALUES(2,'ready',now(),decode(repeat('00',32),'hex')); INSERT INTO search_user_profile_generation_checkpoint(generation,journal_offset,snapshot_phase,evidence_count,evidence_first_offset,evidence_last_offset,evidence_digest) VALUES(2,0,'replay',1,1,1,decode(repeat('00',32),'hex'))`)
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `SELECT active_generation FROM search_user_profile_generation_route WHERE singleton=true FOR UPDATE`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := PromoteGeneration(ctx, pool, 2); done <- err }()
	select {
	case err := <-done:
		t.Fatalf("promotion escaped route lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	route, err := LoadGenerationRoute(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, uint64(2), route.Active)
}

func TestGenerationRecovery_ReopensCatchesUpAndPromotes(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "search_projection_generation_recovery", filepath.Join(searchProjectionRepoRoot(t), "src", "backend", "migrations", "search_db", "000001_init.up.sql"))
	for _, migration := range []string{"000003_space_lifecycle.up.sql", "000002_verification_type.up.sql", "000004_user_profile_projection.up.sql", "000005_user_profile_projection_snapshot.up.sql", "000006_user_profile_projection_quarantine.up.sql", "000007_user_profile_projection_fence.up.sql", "000008_user_profile_projection_generations.up.sql"} {
		integrationtest.ApplySQLFile(t, ctx, pool, searchProjectionRepoRoot(t), filepath.Join("src", "backend", "migrations", "search_db", migration))
	}
	_, err := pool.Exec(ctx, `INSERT INTO search_user_profile_generations(generation,state,ready_at,evidence_sha256) VALUES(2,'ready',now(),decode(repeat('00',32),'hex')); INSERT INTO search_user_profile_generation_checkpoint(generation,journal_offset,snapshot_phase,snapshot_high_watermark,evidence_count,evidence_first_offset,evidence_last_offset,evidence_digest) VALUES(2,2,'replay',0,1,2,2,decode(repeat('00',32),'hex'))`)
	require.NoError(t, err)
	require.NoError(t, ReopenGeneration(ctx, pool, 2))
	profileID, accountID := uuid.NewString(), uuid.NewString()
	event := &userv1.SearchProfileProjectionEvent{ProtocolVersion: 1, EventId: uuid.NewString(), ProfileId: profileID, SourceRevision: 1, JournalOffset: 3, Payload: &userv1.SearchProfileProjectionEvent_Upsert{Upsert: &userv1.SearchProfileUpsert{AccountId: accountID, Username: "recovery", Discriminator: "0001", DisplayName: "Recovery", UsernameSearchKey: "recovery", DisplayNameSearchKey: "recovery", NormalizationVersion: 1}}}
	_, err = (&StoreAdapter{Pool: pool, Generation: 2}).ApplyAndCheckpoint(ctx, event, 3)
	require.NoError(t, err)
	require.NoError(t, MarkGenerationReady(ctx, pool, 2))
	route, err := PromoteGeneration(ctx, pool, 2)
	require.NoError(t, err)
	require.Equal(t, uint64(2), route.Active)
}

func searchProjectionRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}
