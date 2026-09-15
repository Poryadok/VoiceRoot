package providerlifecycle

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/integrationtest"
)

func fakeCommand(kind eventsv1.SubscriptionAggregateKind) Command {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := Command{Provider: "fake", EventID: uuid.NewString(), SubscriptionID: uuid.NewString(), Version: 1,
		Kind: kind, AggregateID: uuid.NewString(), Reason: eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED,
		EffectiveAt: start, PeriodStart: start, PeriodEnd: start.AddDate(0, 1, 0), BillingPeriod: "monthly", RawBody: []byte(`{"version":1}`)}
	if kind == eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE {
		c.PurchaserID = uuid.NewString()
	}
	return c
}

func nextCommand(c Command, reason eventsv1.EntitlementReason, effective time.Time) Command {
	c.EventID = uuid.NewString()
	c.Version++
	c.Reason = reason
	c.EffectiveAt = effective
	return c
}

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "../../../migrations/subscription_db")
	pool := integrationtest.StartPostgres(t, context.Background(), "provider_lifecycle", filepath.Join(root, "000001_init.up.sql"))
	for _, name := range []string{"000002_grace_reminders.up.sql", "000003_space_lifecycle_provider_dedup.up.sql", "000004_entitlement_outbox.up.sql", "000005_provider_lifecycle.up.sql"} {
		migration, err := os.ReadFile(filepath.Join(root, name))
		require.NoError(t, err)
		_, err = pool.Exec(context.Background(), string(migration))
		require.NoError(t, err)
	}
	return pool
}

func TestProviderReplayOrderAndConflictingBody(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL)
	first, err := store.Apply(ctx, c)
	require.NoError(t, err)
	require.Equal(t, "APPLIED", first.Disposition)
	renew := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED, c.PeriodEnd)
	renew.Version = 3
	renew.PeriodStart = c.PeriodEnd
	renew.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	renewed, err := store.Apply(ctx, renew)
	require.NoError(t, err)
	require.EqualValues(t, 2, renewed.Event.AggregateRevision)
	replay, err := store.Apply(ctx, c)
	require.NoError(t, err)
	require.True(t, proto.Equal(first.Event, replay.Event), "replay returns original saved outcome even after renewal")
	require.Equal(t, first.Disposition, replay.Disposition)
	stale := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, c.PeriodEnd)
	old, err := store.Apply(ctx, stale)
	require.NoError(t, err)
	require.Equal(t, "STALE", old.Disposition)
	require.True(t, proto.Equal(renewed.Event, old.Event))
	c.RawBody = []byte(`{"version":999}`)
	_, err = store.Apply(ctx, c)
	require.ErrorIs(t, err, ErrContractMismatch)
	_, err = store.Apply(ctx, c)
	require.ErrorIs(t, err, ErrContractMismatch)
	var conflicts int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_provider_conflicts").Scan(&conflicts))
	require.Equal(t, 1, conflicts, "same rejected request is durably quarantined once")
	var version int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT provider_version FROM subscription_provider_bindings WHERE provider=$1 AND provider_subscription_id=$2", c.Provider, c.SubscriptionID).Scan(&version))
	require.EqualValues(t, 3, version)
	renew.PeriodEnd = renew.PeriodEnd.Add(time.Hour)
	_, err = store.Apply(ctx, renew)
	require.ErrorIs(t, err, ErrContractMismatch, "normalized input is bound even if raw body unchanged")
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 2, count)
	// A different delivery ID cannot disguise conflicting facts at one provider version.
	renew.EventID = uuid.NewString()
	_, err = store.Apply(ctx, renew)
	require.ErrorIs(t, err, ErrContractMismatch)
	var saved []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT payload FROM subscription_entitlement_aggregates").Scan(&saved))
	current := new(eventsv1.SubscriptionStreamEvent)
	require.NoError(t, proto.Unmarshal(saved, current))
	require.True(t, proto.Equal(renewed.Event, current))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_provider_conflicts").Scan(&conflicts))
	require.Equal(t, 3, conflicts)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 2, count)
}

func TestUnchangedOutcomeReplayReturnsHistoricalSnapshot(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(1)
	_, err := store.Apply(ctx, c)
	require.NoError(t, err)
	failure := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, c.PeriodEnd)
	_, err = store.Apply(ctx, failure)
	require.NoError(t, err)
	again := nextCommand(failure, failure.Reason, failure.EffectiveAt.Add(time.Hour))
	old, err := store.Apply(ctx, again)
	require.NoError(t, err)
	require.Equal(t, "UNCHANGED", old.Disposition)
	recover := nextCommand(again, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_RECOVERED, again.EffectiveAt.Add(time.Hour))
	recover.PeriodStart = c.PeriodEnd
	recover.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	newer, err := store.Apply(ctx, recover)
	require.NoError(t, err)
	require.Greater(t, newer.Event.AggregateRevision, old.Event.AggregateRevision)
	replay, err := store.Apply(ctx, again)
	require.NoError(t, err)
	require.Equal(t, "UNCHANGED", replay.Disposition)
	require.True(t, proto.Equal(old.Event, replay.Event))
}

func TestNewProviderBindingRequiresAnUnoccupiedStart(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(1)
	unknown := c
	unknown.Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED
	_, err := store.Apply(ctx, unknown)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	_, err = store.Apply(ctx, c)
	require.NoError(t, err)
	other := fakeCommand(1)
	other.AggregateID = c.AggregateID
	_, err = store.Apply(ctx, other)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	for _, table := range []string{"subscription_provider_bindings", "subscription_provider_outcomes", "subscription_event_outbox"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Equal(t, 1, count)
	}
}

func TestFutureProviderFactWaitsForDatabaseTime(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(1)
	c.EffectiveAt = time.Now().UTC().Add(24 * time.Hour)
	c.PeriodStart = c.EffectiveAt
	c.PeriodEnd = c.EffectiveAt.AddDate(0, 1, 0)
	_, err := store.Apply(ctx, c)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	for _, table := range []string{"subscription_provider_bindings", "subscription_provider_outcomes", "subscription_entitlement_aggregates", "subscription_event_outbox"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count)
	}
	// A valid currently paid aggregate is not revoked by a future period-end fact.
	c.EffectiveAt = time.Now().UTC().Add(-time.Hour)
	c.PeriodStart = c.EffectiveAt
	c.PeriodEnd = time.Now().UTC().Add(24 * time.Hour)
	initial, err := store.Apply(ctx, c)
	require.NoError(t, err)
	futureEnd := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PERIOD_ENDED, c.PeriodEnd)
	_, err = store.Apply(ctx, futureEnd)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	var body []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT payload FROM subscription_entitlement_aggregates").Scan(&body))
	current := new(eventsv1.SubscriptionStreamEvent)
	require.NoError(t, proto.Unmarshal(body, current))
	require.True(t, proto.Equal(initial.Event, current))
	var version, count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT provider_version FROM subscription_provider_bindings").Scan(&version))
	require.Equal(t, 1, version)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 1, count)
}

func TestConcurrentProviderReplayCommitsOneEvent(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(1)
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make(chan Result, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); <-start; r, e := store.Apply(ctx, c); results <- r; errs <- e }()
	}
	close(start)
	wg.Wait()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	a, b := <-results, <-results
	require.True(t, proto.Equal(a.Event, b.Event))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 1, count)
}

func TestProviderFailureRollsBackJournalAndProjection(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(1)
	// Real DB constraint failure occurs after transition preparation. Retrying the
	// exact request must still be able to commit its first outcome.
	_, err := pool.Exec(ctx, "ALTER TABLE subscription_event_outbox ADD CONSTRAINT injected_failure CHECK (false)")
	require.NoError(t, err)
	_, err = store.Apply(ctx, c)
	require.Error(t, err)
	for _, table := range []string{"subscription_provider_outcomes", "subscription_provider_bindings", "subscription_entitlement_aggregates"} {
		var count int
		require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count))
		require.Zero(t, count)
	}
	_, err = pool.Exec(ctx, "ALTER TABLE subscription_event_outbox DROP CONSTRAINT injected_failure")
	require.NoError(t, err)
	r, err := store.Apply(ctx, c)
	require.NoError(t, err)
	require.EqualValues(t, 1, r.Event.AggregateRevision)
}

func TestRepurchaseKeepsRevisionAndRetiresOldProviderBinding(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(1)
	first, err := store.Apply(ctx, c)
	require.NoError(t, err)
	cancel := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_CANCEL_SCHEDULED, c.EffectiveAt.Add(time.Hour))
	_, err = store.Apply(ctx, cancel)
	require.NoError(t, err)
	expiry := nextCommand(cancel, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PERIOD_ENDED, c.PeriodEnd)
	_, err = store.Apply(ctx, expiry)
	require.NoError(t, err)
	// A reused provider identity does not prove a distinct new purchase.
	reuse := nextCommand(expiry, eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED, c.PeriodEnd)
	reuse.PeriodStart = c.PeriodEnd
	reuse.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	_, err = store.Apply(ctx, reuse)
	require.ErrorIs(t, err, ErrNeedsReconciliation)
	purchase := fakeCommand(1)
	purchase.AggregateID = c.AggregateID
	purchase.EffectiveAt = c.PeriodEnd
	purchase.PeriodStart = c.PeriodEnd
	purchase.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	bought, err := store.Apply(ctx, purchase)
	require.NoError(t, err)
	require.EqualValues(t, 4, bought.Event.AggregateRevision)
	require.Equal(t, first.Event.GetEntitlementChanged().DeletionFenceId, bought.Event.GetEntitlementChanged().DeletionFenceId)
	require.Nil(t, bought.Event.GetEntitlementChanged().DowngradeCycleId)
	late := nextCommand(expiry, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED, c.PeriodEnd)
	late.PeriodStart = c.PeriodEnd
	late.PeriodEnd = purchase.PeriodEnd
	old, err := store.Apply(ctx, late)
	require.NoError(t, err)
	require.Equal(t, "RETIRED", old.Disposition)
	require.True(t, proto.Equal(bought.Event, old.Event))
	lateConflict := late
	lateConflict.EventID = uuid.NewString()
	lateConflict.BillingPeriod = "yearly"
	_, err = store.Apply(ctx, lateConflict)
	require.ErrorIs(t, err, ErrContractMismatch, "new retired version still binds its exact authoritative facts")
	conflict := expiry
	conflict.EventID = uuid.NewString()
	conflict.BillingPeriod = "yearly"
	_, err = store.Apply(ctx, conflict)
	require.ErrorIs(t, err, ErrContractMismatch, "retirement cannot hide same-version conflicting facts")
	stolen := nextCommand(purchase, eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED, purchase.EffectiveAt)
	stolen.AggregateID = uuid.NewString()
	_, err = store.Apply(ctx, stolen)
	require.ErrorIs(t, err, ErrContractMismatch, "provider subscription cannot move targets")
}

func TestRepeatedFailureAdvancesOnlyProviderOrder(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(2)
	_, err := store.Apply(ctx, c)
	require.NoError(t, err)
	failure := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, c.PeriodEnd)
	grace, err := store.Apply(ctx, failure)
	require.NoError(t, err)
	again := nextCommand(failure, failure.Reason, failure.EffectiveAt.Add(48*time.Hour))
	r, err := store.Apply(ctx, again)
	require.NoError(t, err)
	require.Equal(t, "UNCHANGED", r.Disposition)
	require.True(t, proto.Equal(grace.Event, r.Event))
	var version int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT provider_version FROM subscription_provider_bindings").Scan(&version))
	require.EqualValues(t, 3, version)
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 2, count)
}

func TestConcurrentFailureAndRenewalCannotRegress(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	store := Store{Pool: pool}
	c := fakeCommand(2)
	_, err := store.Apply(ctx, c)
	require.NoError(t, err)
	failure := nextCommand(c, eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, c.PeriodEnd)
	renew := nextCommand(failure, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED, c.PeriodEnd.Add(time.Hour))
	renew.PeriodStart = c.PeriodEnd
	renew.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, cmd := range []Command{failure, renew} {
		go func() { <-start; _, e := store.Apply(ctx, cmd); errs <- e }()
	}
	close(start)
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	var body []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT payload FROM subscription_entitlement_aggregates").Scan(&body))
	ev := new(eventsv1.SubscriptionStreamEvent)
	require.NoError(t, proto.Unmarshal(body, ev))
	require.Equal(t, eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE, ev.GetEntitlementChanged().State)
	require.Equal(t, renew.PeriodEnd, ev.GetEntitlementChanged().EntitledUntil.AsTime())
	var version, count int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT provider_version FROM subscription_provider_bindings").Scan(&version))
	require.EqualValues(t, 3, version)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.EqualValues(t, ev.AggregateRevision, count)
}
