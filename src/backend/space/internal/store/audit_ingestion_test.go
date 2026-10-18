package store

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
	"voice/backend/pkg/spaceaudit"
)

func applyAuditIngestionMigration(t *testing.T, st *SpaceStore) {
	t.Helper()
	for _, name := range []string{"000025_tree_audit_actions.up.sql", "000027_audit_ingestion.up.sql"} {
		body, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", name))
		require.NoError(t, err)
		_, err = st.Pool.Exec(context.Background(), string(body))
		require.NoError(t, err)
	}
}

func importFixture(t *testing.T, st *SpaceStore) spaceaudit.ReplayEvent {
	t.Helper()
	owner := uuid.New()
	space, err := st.CreateSpace(context.Background(), owner, "Audit imports", "", "private")
	require.NoError(t, err)
	return spaceaudit.ReplayEvent{AuditEventID: uuid.NewString(), SpaceID: space.ID.String(), ActorProfileID: owner.String(),
		Action: "role_created", TargetType: "role", TargetID: uuid.NewString(), DetailsJSON: "{}",
		OccurredAt: timestamppb.New(time.Now().UTC().Add(-366 * 24 * time.Hour))}
}

func assertImportCounts(t *testing.T, st *SpaceStore, id string, audit, outbox, receipt int) {
	t.Helper()
	for table, want := range map[string]int{"audit_log": audit, "audit_outbox": outbox, "audit_event_receipts": receipt} {
		key := "audit_event_id"
		if table == "audit_log" {
			key = "id"
		}
		var got int
		require.NoError(t, st.Pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE "+key+"=$1", id).Scan(&got))
		require.Equal(t, want, got, table)
	}
}

func TestAuditIngestionReplayRetentionAndLocalFence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	event := importFixture(t, st)
	event.OccurredAt.Nanos = 123456001 // both variants persist as the same PostgreSQL microsecond
	localID := uuid.New()
	insertAuditFixture(t, st, localID, uuid.MustParse(event.SpaceID), uuid.MustParse(event.ActorProfileID), "member_banned", time.Now().UTC())
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	require.NoError(t, st.ImportAuditEvent(ctx, "role", event))
	require.NoError(t, st.ImportAuditEvent(ctx, "role", event), "exact retry uses same persisted effect")
	assertImportCounts(t, st, event.AuditEventID, 1, 1, 1)
	changed := event
	changed.OccurredAt = &timestamppb.Timestamp{Seconds: event.OccurredAt.Seconds, Nanos: event.OccurredAt.Nanos + 1}
	if changed.OccurredAt.Nanos == 1000000000 {
		changed.OccurredAt.Seconds++
		changed.OccurredAt.Nanos = 0
	}
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", changed), ErrAuditEventConflict)
	changed = event
	changed.DetailsJSON = "{}\n"
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", changed), spaceaudit.ErrInvalidDetails)
	changed = event
	changed.AuditEventID = localID.String()
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", changed), ErrAuditEventConflict)
	_, err := st.Pool.Exec(ctx, `UPDATE audit_outbox SET delivered_at=clock_timestamp() WHERE audit_event_id=$1`, event.AuditEventID)
	require.NoError(t, err)
	removed, err := st.CleanupExpiredAuditLog(ctx, 10)
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	assertImportCounts(t, st, event.AuditEventID, 0, 0, 1)
	require.NoError(t, st.ImportAuditEvent(ctx, "role", event), "retained receipt acknowledges without resurrection")
	assertImportCounts(t, st, event.AuditEventID, 0, 0, 1)
	changed = event
	changed.TargetID = uuid.NewString()
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", changed), ErrAuditEventConflict)
	_, err = st.Pool.Exec(ctx, `DELETE FROM audit_event_receipts WHERE audit_event_id=$1`, event.AuditEventID)
	require.Error(t, err, "ordinary callers cannot erase replay fences")
	require.Error(t, st.DeleteSpace(ctx, uuid.MustParse(event.SpaceID)), "existing hard-delete guard remains")
}

func TestAuditIngestionMigrationRoundTripAndUint64(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	down, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000027_audit_ingestion.down.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(down))
	require.NoError(t, err, "empty receipt migration can be reversed without losing reservations")
	up, err := os.ReadFile(filepath.Join(repoRoot(t), "src", "backend", "migrations", "space_db", "000027_audit_ingestion.up.sql"))
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, string(up))
	require.NoError(t, err, "roundtrip must restore original local registry without recursive helper")
	event := importFixture(t, st)
	event.Action = "chat_override_set"
	event.TargetType = "chat"
	event.DetailsJSON, err = spaceaudit.OverrideDetails(event.Action, uuid.NewString(), math.MaxUint64, math.MaxUint64)
	require.NoError(t, err)
	require.NoError(t, st.ImportAuditEvent(ctx, "role", event))
	var allow, deny string
	require.NoError(t, st.Pool.QueryRow(ctx, `SELECT details->>'allow_mask',details->>'deny_mask' FROM audit_log WHERE id=$1`, event.AuditEventID).Scan(&allow, &deny))
	require.Equal(t, "18446744073709551615", allow)
	require.Equal(t, allow, deny)
	connection, err := st.Pool.Acquire(ctx)
	require.NoError(t, err)
	_, err = connection.Exec(ctx, string(down))
	require.Error(t, err, "rollback may not erase any reserved ID")
	// The migration's failed explicit transaction must be ended before readback.
	_, err = connection.Exec(ctx, "ROLLBACK")
	require.NoError(t, err)
	connection.Release()
	assertImportCounts(t, st, event.AuditEventID, 1, 1, 1)
}

func TestAuditIngestionLocalExternalCollisionRaceAndRetainedLocalFence(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	event := importFixture(t, st)
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	start := make(chan struct{})
	external := make(chan error, 1)
	local := make(chan error, 1)
	go func() { <-start; external <- st.ImportAuditEvent(ctx, "role", event) }()
	go func() {
		<-start
		_, err := st.Pool.Exec(ctx, `INSERT INTO audit_log(id,space_id,actor_profile_id,action,target_type,target_id,details,created_at)
 VALUES($1,$2,$3,'member_banned','account',$4,'{}'::jsonb,$5)`, event.AuditEventID, event.SpaceID, event.ActorProfileID, uuid.New(), event.OccurredAt.AsTime())
		local <- err
	}()
	close(start)
	extErr, localErr := <-external, <-local
	if extErr == nil {
		require.Error(t, localErr)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, localErr, &pgErr)
		require.Equal(t, "23505", pgErr.Code)
	} else {
		require.ErrorIs(t, extErr, ErrAuditEventConflict)
		require.NoError(t, localErr)
	}
	assertImportCounts(t, st, event.AuditEventID, 1, 1, 1)
	// A separately authored local fact must keep its global ID even after payload retention.
	localID := uuid.New()
	insertAuditFixture(t, st, localID, uuid.MustParse(event.SpaceID), uuid.MustParse(event.ActorProfileID), "member_banned", event.OccurredAt.AsTime())
	_, err := st.Pool.Exec(ctx, `UPDATE audit_outbox SET delivered_at=clock_timestamp() WHERE audit_event_id=$1`, localID)
	require.NoError(t, err)
	_, err = st.CleanupExpiredAuditLog(ctx, 10)
	require.NoError(t, err)
	assertImportCounts(t, st, localID.String(), 0, 0, 1)
	event.AuditEventID = localID.String()
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", event), ErrAuditEventConflict)
}

func TestAuditIngestionConcurrentExactAndConflict(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	event := importFixture(t, st)
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	start := make(chan struct{})
	results := make(chan error, 8)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; results <- st.ImportAuditEvent(ctx, "role", event) }()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	assertImportCounts(t, st, event.AuditEventID, 1, 1, 1)
	first, second := event, event
	first.AuditEventID = uuid.NewString()
	second.AuditEventID = first.AuditEventID
	second.TargetID = uuid.NewString()
	start = make(chan struct{})
	results = make(chan error, 2)
	for _, candidate := range []spaceaudit.ReplayEvent{first, second} {
		wg.Add(1)
		go func(e spaceaudit.ReplayEvent) {
			defer wg.Done()
			<-start
			results <- st.ImportAuditEvent(ctx, "role", e)
		}(candidate)
	}
	close(start)
	wg.Wait()
	close(results)
	winners, conflicts := 0, 0
	for err := range results {
		if err == nil {
			winners++
		} else {
			require.ErrorIs(t, err, ErrAuditEventConflict)
			conflicts++
		}
	}
	require.Equal(t, 1, winners)
	require.Equal(t, 1, conflicts)
	assertImportCounts(t, st, first.AuditEventID, 1, 1, 1)
}

func TestAuditIngestionInvalidAndFailedEffectsReserveNothing(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	event := importFixture(t, st)
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	bad := event
	bad.DetailsJSON = `{"private":"not allowed"}`
	require.Error(t, st.ImportAuditEvent(ctx, "role", bad))
	assertImportCounts(t, st, event.AuditEventID, 0, 0, 0)
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "chat", event), spaceaudit.ErrProducerAction)
	assertImportCounts(t, st, event.AuditEventID, 0, 0, 0)
	_, err := st.Pool.Exec(ctx, `CREATE FUNCTION fail_import_outbox() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected effect failure'; END $$;
CREATE TRIGGER zz_fail_import_outbox AFTER INSERT ON audit_outbox FOR EACH ROW EXECUTE FUNCTION fail_import_outbox()`)
	require.NoError(t, err)
	require.Error(t, st.ImportAuditEvent(ctx, "role", event))
	assertImportCounts(t, st, event.AuditEventID, 0, 0, 0)
}

func TestAuditIngestionCompensationPreservesReservation(t *testing.T) {
	st := auditLedgerStoreFixture(t)
	applyAuditIngestionMigration(t, st)
	ctx := context.Background()
	previous, next := uuid.New(), uuid.New()
	space, err := st.CreateSpace(ctx, previous, "Compensation receipt", "", "private")
	require.NoError(t, err)
	_, err = st.Pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id) VALUES($1,$2)`, space.ID, next)
	require.NoError(t, err)
	require.NoError(t, st.TransferOwnership(ctx, space.ID, previous, next))
	id := uuid.New()
	const token = "1111111111111111111111111111111111111111111111111111111111111111"
	legacy := st.LegacyOwnershipLeaseStoreForTest()
	require.NoError(t, legacy.RecordOwnershipTransferred(ctx, id, space.ID, previous, next, token))
	require.Error(t, legacy.DeleteAuditLogEntry(ctx, id, "2222222222222222222222222222222222222222222222222222222222222222"))
	assertImportCounts(t, st, id.String(), 1, 1, 1)
	require.NoError(t, legacy.DeleteAuditLogEntry(ctx, id, token))
	assertImportCounts(t, st, id.String(), 0, 0, 1)
	event := spaceaudit.ReplayEvent{AuditEventID: id.String(), SpaceID: space.ID.String(), ActorProfileID: previous.String(), Action: "role_created", TargetType: "role", TargetID: uuid.NewString(), DetailsJSON: "{}", OccurredAt: timestamppb.Now()}
	require.ErrorIs(t, st.ImportAuditEvent(ctx, "role", event), ErrAuditEventConflict)
}
