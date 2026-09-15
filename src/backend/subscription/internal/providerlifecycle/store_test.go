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
	pool := integrationtest.StartPostgres(t, context.Background(), "provider_lifecycle", filepath.Join(root, "000004_entitlement_outbox.up.sql"))
	migration, err := os.ReadFile(filepath.Join(root, "000005_provider_lifecycle.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), string(migration))
	require.NoError(t, err)
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
	renew.PeriodEnd = renew.PeriodEnd.Add(time.Hour)
	_, err = store.Apply(ctx, renew)
	require.ErrorIs(t, err, ErrContractMismatch, "normalized input is bound even if raw body unchanged")
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 2, count)
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
	purchase := fakeCommand(1)
	purchase.AggregateID = c.AggregateID
	purchase.EffectiveAt = c.PeriodEnd
	purchase.PeriodStart = c.PeriodEnd
	purchase.PeriodEnd = c.PeriodEnd.AddDate(0, 1, 0)
	bought, err := store.Apply(ctx, purchase)
	require.NoError(t, err)
	require.EqualValues(t, 4, bought.Event.AggregateRevision)
	require.Equal(t, first.Event.GetEntitlementChanged().DeletionFenceId, bought.Event.GetEntitlementChanged().DeletionFenceId)
	require.NotEqual(t, first.Event.GetEntitlementChanged().EntitlementId, bought.Event.GetEntitlementChanged().EntitlementId)
	require.Nil(t, bought.Event.GetEntitlementChanged().DowngradeCycleId)
	late := nextCommand(expiry, eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED, c.PeriodEnd)
	late.PeriodStart = c.PeriodEnd
	late.PeriodEnd = purchase.PeriodEnd
	old, err := store.Apply(ctx, late)
	require.NoError(t, err)
	require.Equal(t, "STALE", old.Disposition)
	require.True(t, proto.Equal(bought.Event, old.Event))
	stolen := nextCommand(purchase, eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED, purchase.EffectiveAt)
	stolen.AggregateID = uuid.NewString()
	_, err = store.Apply(ctx, stolen)
	require.ErrorIs(t, err, ErrContractMismatch, "provider subscription cannot move targets")
}
