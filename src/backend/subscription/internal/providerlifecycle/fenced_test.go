package providerlifecycle

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/subscription/internal/currentauthority"
)

func fencedDB(t *testing.T, seeds ...Command) (*pgxpool.Pool, *pgxpool.Pool, string) {
	t.Helper()
	admin := testDB(t)
	_, file, _, _ := runtime.Caller(0)
	for _, name := range []string{"000007_authority_fence", "000008_kernel_writes"} {
		body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/subscription_db", name+".up.sql"))
		require.NoError(t, err)
		_, err = admin.Exec(context.Background(), string(body))
		require.NoError(t, err)
	}
	_, err := admin.Exec(context.Background(), `CREATE ROLE old_runtime LOGIN PASSWORD 'fixture-only';
CREATE ROLE kernel_writer LOGIN PASSWORD 'fixture-only'; CREATE ROLE kernel_reader LOGIN PASSWORD 'fixture-only'`)
	require.NoError(t, err)
	for _, c := range seeds {
		_, err = (Store{Pool: admin}).Apply(context.Background(), c)
		require.NoError(t, err)
	}
	generation := uuid.NewString()
	require.NoError(t, currentauthority.InstallFence(context.Background(), admin, currentauthority.FenceRequest{
		Generation: generation, SourceCommit: strings.Repeat("a", 40), WriterRole: "kernel_writer", ReaderRole: "kernel_reader", LegacyRoles: []string{"old_runtime"},
	}))
	cfg := admin.Config().Copy()
	cfg.ConnConfig.User, cfg.ConnConfig.Password = "kernel_writer", "fixture-only"
	writer, err := pgxpool.NewWithConfig(context.Background(), cfg)
	require.NoError(t, err)
	t.Cleanup(writer.Close)
	return admin, writer, generation
}

func TestFencedProviderWriteAndReplayDoNotCertifyShadow(t *testing.T) {
	c := fakeCommand(eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL)
	admin, writer, generation := fencedDB(t, c)
	ctx := context.Background()
	// Actual pre-fence-shaped source bytes exist, but no kernel evidence exists.
	_, err := (Store{Pool: writer}).ApplyFenced(ctx, c, generation)
	require.NoError(t, err)
	var count int
	require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM subscription_kernel_writes").Scan(&count))
	require.Zero(t, count, "historical replay must never stamp shadow bytes")
	renew := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED, c.PeriodEnd)
	renew.PeriodStart, renew.PeriodEnd = c.PeriodEnd, c.PeriodEnd.AddDate(0, 1, 0)
	result, err := (Store{Pool: writer}).ApplyFenced(ctx, renew, generation)
	require.NoError(t, err)
	require.EqualValues(t, 2, result.Event.AggregateRevision)
	var recordedGeneration, state string
	require.NoError(t, admin.QueryRow(ctx, "SELECT generation::text,state FROM subscription_kernel_writes").Scan(&recordedGeneration, &state))
	require.Equal(t, generation, recordedGeneration)
	require.Equal(t, "WRITTEN", state, "writer provenance alone is never CURRENT")
	_, err = (Store{Pool: writer}).ApplyFenced(ctx, c, uuid.NewString())
	require.ErrorIs(t, err, currentauthority.ErrFenceConflict, "even historical replay must respect the current generation")
	_, err = (Store{Pool: admin}).ApplyFenced(ctx, renew, generation)
	require.ErrorIs(t, err, currentauthority.ErrUnsafeRole, "installer credentials cannot serve kernel requests")
	_, err = (Store{Pool: writer}).Apply(ctx, renew)
	require.ErrorIs(t, err, currentauthority.ErrFenceConflict, "legacy entry cannot bypass generation on the new login")
}

func TestFencedProviderEvidenceFailureRollsBackEveryWrite(t *testing.T) {
	admin, writer, generation := fencedDB(t)
	ctx := context.Background()
	_, err := admin.Exec(ctx, `CREATE FUNCTION fixture_reject_kernel() RETURNS trigger LANGUAGE plpgsql AS $$BEGIN RAISE EXCEPTION 'fixture rollback'; END$$;
CREATE TRIGGER fixture_reject_kernel BEFORE INSERT ON subscription_kernel_writes FOR EACH ROW EXECUTE FUNCTION fixture_reject_kernel()`)
	require.NoError(t, err)
	c := fakeCommand(eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL)
	_, err = (Store{Pool: writer}).ApplyFenced(ctx, c, generation)
	require.Error(t, err)
	for _, table := range []string{"subscription_provider_bindings", "subscription_provider_versions", "subscription_provider_outcomes", "subscription_entitlement_aggregates", "subscription_event_outbox", "subscription_grace_reminders", "subscription_kernel_writes"} {
		var count int
		require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count, table)
	}
}

func TestFencedWriterHoldsGenerationAndRejectsPrivilegeDrift(t *testing.T) {
	admin, writer, generation := fencedDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	tx, err := writer.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(context.Background()) }()
	_, err = currentauthority.AcquireWriter(ctx, tx, generation)
	require.NoError(t, err)
	var pid uint32
	require.NoError(t, tx.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&pid))
	installer, err := admin.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = installer.Rollback(context.Background()) }()
	finished := make(chan error, 1)
	go func() { _, e := installer.Exec(ctx, "SELECT pg_advisory_xact_lock(17006)"); finished <- e }()
	require.Eventually(t, func() bool {
		var blocked bool
		e := admin.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)))", pid).Scan(&blocked)
		return e == nil && blocked
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-finished)
	require.NoError(t, installer.Rollback(ctx))
	_, err = admin.Exec(ctx, "GRANT UPDATE ON subscription_authority_fence TO kernel_writer")
	require.NoError(t, err)
	c := fakeCommand(eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL)
	_, err = (Store{Pool: writer}).ApplyFenced(ctx, c, generation)
	require.ErrorIs(t, err, currentauthority.ErrUnsafeRole, "fence self-certification drift must fail before domain writes")
}

func TestFencedWriterDeadlineDoesNotCrossGenerationWait(t *testing.T) {
	admin, writer, generation := fencedDB(t)
	ctx := context.Background()
	installer, err := admin.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = installer.Rollback(ctx) }()
	_, err = installer.Exec(ctx, "SELECT pg_advisory_xact_lock(17006)")
	require.NoError(t, err)
	bounded, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	c := fakeCommand(eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL)
	_, err = (Store{Pool: writer}).ApplyFenced(bounded, c, generation)
	require.Error(t, err)
	require.ErrorIs(t, bounded.Err(), context.DeadlineExceeded)
	require.NoError(t, installer.Rollback(ctx))
	var count int
	require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM subscription_provider_outcomes").Scan(&count))
	require.Zero(t, count, "a timed-out generation wait never creates a replay receipt or grant")
}
