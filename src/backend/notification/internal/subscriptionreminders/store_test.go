package subscriptionreminders

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/integrationtest"
)

func fixture(t *testing.T) (context.Context, Store) {
	t.Helper()
	if testing.Short() {
		t.Skip("real PostgreSQL required")
	}
	ctx := context.Background()
	pool := integrationtest.StartPostgres(t, ctx, "reminders", "")
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations", "notification_db")
	sql, err := os.ReadFile(filepath.Join(root, "000005_subscription_reminders.up.sql"))
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err)
	return ctx, Store{Pool: pool}
}

func grace(t *testing.T, s Store, ctx context.Context) *eventsv1.SubscriptionStreamEvent {
	t.Helper()
	var now time.Time
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now))
	account := uuid.NewString()
	return &eventsv1.SubscriptionStreamEvent{EventId: uuid.NewString(), ProtocolVersion: 1, AggregateKind: eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL, AggregateId: account, AggregateRevision: 2, OccurredAt: timestamppb.New(now), Payload: &eventsv1.SubscriptionStreamEvent_EntitlementChanged{EntitlementChanged: &eventsv1.EntitlementChanged{EntitlementId: uuid.NewString(), Plan: "premium", AccountId: account, DeletionFenceId: uuid.NewString(), State: eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD, Reason: eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED, EffectiveAt: timestamppb.New(now.Add(-time.Hour)), CurrentPeriodEnd: timestamppb.New(now.Add(-time.Hour)), GracePeriodEnd: timestamppb.New(now.Add(7*24*time.Hour - time.Hour)), EntitledUntil: timestamppb.New(now.Add(7*24*time.Hour - time.Hour))}}}
}

func reminder(s *eventsv1.SubscriptionStreamEvent, day int32) *eventsv1.SubscriptionStreamEvent {
	e := proto.Clone(s).(*eventsv1.SubscriptionStreamEvent)
	e.EventId = uuid.NewString()
	e.Payload = &eventsv1.SubscriptionStreamEvent_GraceReminder{GraceReminder: &eventsv1.GraceReminder{AccountId: s.AggregateId, Plan: "premium", Day: day}}
	return e
}
func apply(t *testing.T, ctx context.Context, s Store, e *eventsv1.SubscriptionStreamEvent) (string, error) {
	t.Helper()
	raw, err := proto.Marshal(e)
	require.NoError(t, err)
	return s.Apply(ctx, raw)
}

func TestAheadReminderRetriesThenCommitsUniqueChannelWork(t *testing.T) {
	ctx, s := fixture(t)
	snapshot := grace(t, s, ctx)
	e := reminder(snapshot, 1)
	_, err := apply(t, ctx, s, e)
	require.ErrorIs(t, err, ErrAhead)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM notification_subscription_inbox").Scan(&count))
	require.Zero(t, count)
	outcome, err := apply(t, ctx, s, snapshot)
	require.NoError(t, err)
	require.Equal(t, "APPLIED", outcome)
	outcome, err = apply(t, ctx, s, e)
	require.NoError(t, err)
	require.Equal(t, "QUEUED", outcome)
	outcome, err = apply(t, ctx, Store{Pool: s.Pool}, e)
	require.NoError(t, err)
	require.Equal(t, "QUEUED", outcome)
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM notification_reminder_dispatch").Scan(&count))
	require.Equal(t, 2, count)
	e.GetGraceReminder().Day = 3
	_, err = apply(t, ctx, s, e)
	require.ErrorIs(t, err, ErrContractMismatch)
}

func TestSuccessorSuppressesPendingAndStaleReminder(t *testing.T) {
	ctx, s := fixture(t)
	snapshot := grace(t, s, ctx)
	_, err := apply(t, ctx, s, snapshot)
	require.NoError(t, err)
	e := reminder(snapshot, 1)
	_, err = apply(t, ctx, s, e)
	require.NoError(t, err)
	recovered := proto.Clone(snapshot).(*eventsv1.SubscriptionStreamEvent)
	recovered.EventId = uuid.NewString()
	recovered.AggregateRevision++
	p := recovered.GetEntitlementChanged()
	p.State = eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE
	p.Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_RECOVERED
	p.EffectiveAt = recovered.OccurredAt
	p.CurrentPeriodEnd = timestamppb.New(time.Now().Add(30 * 24 * time.Hour))
	p.EntitledUntil = p.CurrentPeriodEnd
	p.GracePeriodEnd = nil
	_, err = apply(t, ctx, s, recovered)
	require.NoError(t, err)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM notification_reminder_dispatch WHERE status='SUPPRESSED'").Scan(&count))
	require.Equal(t, 2, count)
	outcome, err := apply(t, ctx, s, reminder(snapshot, 3))
	require.NoError(t, err)
	require.Equal(t, "STALE", outcome)
}

func TestReminderWindowNeverBurstsAndLeasesFenceRestart(t *testing.T) {
	ctx, s := fixture(t)
	snapshot := grace(t, s, ctx)
	p := snapshot.GetEntitlementChanged()
	p.EffectiveAt = timestamppb.New(time.Now().Add(-3 * 24 * time.Hour))
	p.CurrentPeriodEnd = p.EffectiveAt
	p.GracePeriodEnd = timestamppb.New(p.EffectiveAt.AsTime().Add(7 * 24 * time.Hour))
	p.EntitledUntil = p.GracePeriodEnd
	_, err := apply(t, ctx, s, snapshot)
	require.NoError(t, err)
	outcome, err := apply(t, ctx, s, reminder(snapshot, 1))
	require.NoError(t, err)
	require.Equal(t, "SUPPRESSED", outcome)
	_, err = apply(t, ctx, s, reminder(snapshot, 3))
	require.NoError(t, err)
	rows, err := s.Claim(ctx, 2, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	_, err = s.Pool.Exec(ctx, "UPDATE notification_reminder_dispatch SET lease_until=clock_timestamp()-interval '1 second'")
	require.NoError(t, err)
	next, err := Store{Pool: s.Pool}.Claim(ctx, 2, time.Minute)
	require.NoError(t, err)
	require.Len(t, next, 2)
	require.ErrorIs(t, s.Complete(ctx, rows[0]), ErrLeaseLost)
	require.NoError(t, s.Retry(ctx, next[0]))
	require.NoError(t, s.Complete(ctx, next[1]))
}

func TestConcurrentReplayKeepsOneLogicalReminderAndRejectsSecondIdentity(t *testing.T) {
	ctx, s := fixture(t)
	snapshot := grace(t, s, ctx)
	_, err := apply(t, ctx, s, snapshot)
	require.NoError(t, err)
	e := reminder(snapshot, 1)
	raw, err := proto.Marshal(e)
	require.NoError(t, err)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for range 8 {
		wg.Go(func() { _, applyErr := s.Apply(ctx, raw); results <- applyErr })
	}
	wg.Wait()
	close(results)
	for applyErr := range results {
		require.NoError(t, applyErr)
	}
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM notification_reminder_dispatch").Scan(&count))
	require.Equal(t, 2, count)
	conflict := proto.Clone(e).(*eventsv1.SubscriptionStreamEvent)
	conflict.EventId = uuid.NewString()
	_, err = apply(t, ctx, s, conflict)
	require.ErrorIs(t, err, ErrContractMismatch)
	// A different snapshot under the same source revision is also a conflict,
	// even when its event UUID is fresh.
	conflictingSnapshot := proto.Clone(snapshot).(*eventsv1.SubscriptionStreamEvent)
	conflictingSnapshot.EventId = uuid.NewString()
	conflictingSnapshot.GetEntitlementChanged().EntitlementId = uuid.NewString()
	_, err = apply(t, ctx, s, conflictingSnapshot)
	require.ErrorIs(t, err, ErrContractMismatch)
}

func TestSpacePayerDoesNotCollapseDifferentSpacesAndDaySeven(t *testing.T) {
	ctx, s := fixture(t)
	payer := uuid.NewString()
	for range 2 {
		snapshot := grace(t, s, ctx)
		snapshot.AggregateKind = eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE
		snapshot.AggregateId = uuid.NewString()
		p := snapshot.GetEntitlementChanged()
		p.AccountId = ""
		p.SpaceId = snapshot.AggregateId
		p.Plan = "space_pro"
		p.PurchaserAccountId = &payer
		p.EffectiveAt = timestamppb.New(time.Now().Add(-6*24*time.Hour - time.Hour))
		p.CurrentPeriodEnd = p.EffectiveAt
		p.GracePeriodEnd = timestamppb.New(p.EffectiveAt.AsTime().Add(7 * 24 * time.Hour))
		p.EntitledUntil = p.GracePeriodEnd
		_, err := apply(t, ctx, s, snapshot)
		require.NoError(t, err)
		e := reminder(snapshot, 7)
		e.GetGraceReminder().AccountId = payer
		e.GetGraceReminder().Plan = "space_pro"
		e.GetGraceReminder().SpaceId = &snapshot.AggregateId
		outcome, err := apply(t, ctx, s, e)
		require.NoError(t, err)
		require.Equal(t, "QUEUED", outcome)
	}
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(DISTINCT notification_id) FROM notification_reminder_dispatch WHERE payer_account_id=$1", payer).Scan(&count))
	require.Equal(t, 2, count)
	rows, err := s.Claim(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 4)
}

func TestPurchaserPurgedGraceMetadataSuccessorSuppressesPendingWork(t *testing.T) {
	ctx, s := fixture(t)
	snapshot := grace(t, s, ctx)
	payer := snapshot.AggregateId
	snapshot.AggregateKind = eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE
	snapshot.AggregateId = uuid.NewString()
	p := snapshot.GetEntitlementChanged()
	p.AccountId = ""
	p.SpaceId = snapshot.AggregateId
	p.Plan = "space_pro"
	p.PurchaserAccountId = &payer
	_, err := apply(t, ctx, s, snapshot)
	require.NoError(t, err)
	e := reminder(snapshot, 1)
	e.GetGraceReminder().AccountId = payer
	e.GetGraceReminder().Plan = "space_pro"
	e.GetGraceReminder().SpaceId = &snapshot.AggregateId
	_, err = apply(t, ctx, s, e)
	require.NoError(t, err)
	purged := proto.Clone(snapshot).(*eventsv1.SubscriptionStreamEvent)
	purged.EventId = uuid.NewString()
	purged.AggregateRevision++
	p = purged.GetEntitlementChanged()
	p.PurchaserDeleted = true
	p.PurchaserAccountId = nil
	p.Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED
	p.EffectiveAt = purged.OccurredAt // metadata transition time, not grace origin
	outcome, err := apply(t, ctx, s, purged)
	require.NoError(t, err)
	require.Equal(t, "APPLIED", outcome)
	var count int
	require.NoError(t, s.Pool.QueryRow(ctx, "SELECT count(*) FROM notification_reminder_dispatch WHERE status='SUPPRESSED'").Scan(&count))
	require.Equal(t, 2, count)
}
