package entitlementoutbox

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
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/integrationtest"
)

func snapshotEvent() *eventsv1.SubscriptionStreamEvent {
	now := time.Now().UTC().Truncate(time.Microsecond)
	account := uuid.NewString()
	return &eventsv1.SubscriptionStreamEvent{
		EventId: uuid.NewString(), OccurredAt: timestamppb.New(now), ProtocolVersion: 1,
		AggregateKind: eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_PERSONAL,
		AggregateId:   account, AggregateRevision: 1,
		Payload: &eventsv1.SubscriptionStreamEvent_EntitlementChanged{EntitlementChanged: &eventsv1.EntitlementChanged{
			EntitlementId: uuid.NewString(), AccountId: account, Plan: "premium",
			DeletionFenceId: uuid.NewString(),
			State:           eventsv1.EntitlementState_ENTITLEMENT_STATE_ACTIVE,
			Reason:          eventsv1.EntitlementReason_ENTITLEMENT_REASON_STARTED,
			EffectiveAt:     timestamppb.New(now), CurrentPeriodEnd: timestamppb.New(now.Add(24 * time.Hour)),
			EntitledUntil: timestamppb.New(now.Add(24 * time.Hour)),
		}},
	}
}

func database(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("PostgreSQL integration")
	}
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	path := filepath.Join(filepath.Dir(file), "../../../migrations/subscription_db/000004_entitlement_outbox.up.sql")
	_, err := os.Stat(path)
	require.NoError(t, err)
	return integrationtest.StartPostgres(t, context.Background(), "entitlement_outbox", path)
}

func commitEvent(t *testing.T, pool *pgxpool.Pool, expected int64, ev *eventsv1.SubscriptionStreamEvent) {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	require.NoError(t, Append(ctx, tx, expected, ev))
	require.NoError(t, tx.Commit(ctx))
}

func TestAppendAtomicRollbackReplayAndRevision(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	ev := snapshotEvent()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.NoError(t, Append(ctx, tx, 0, ev))
	require.NoError(t, tx.Rollback(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_entitlement_aggregates").Scan(&count))
	require.Zero(t, count)
	commitEvent(t, pool, 0, ev)
	commitEvent(t, pool, 0, ev) // Exact retry cannot allocate a second revision/event.
	var bytes []byte
	require.NoError(t, pool.QueryRow(ctx, "SELECT payload FROM subscription_event_outbox WHERE event_id=$1", ev.EventId).Scan(&bytes))
	want, err := proto.MarshalOptions{Deterministic: true}.Marshal(ev)
	require.NoError(t, err)
	require.Equal(t, want, bytes)
	mutated := proto.Clone(ev).(*eventsv1.SubscriptionStreamEvent)
	mutated.GetEntitlementChanged().CancelAtPeriodEnd = true
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, Append(ctx, tx, 0, mutated), ErrContractMismatch)
	require.NoError(t, tx.Rollback(ctx))
	mutated.EventId = uuid.NewString()
	tx, err = pool.Begin(ctx)
	require.NoError(t, err)
	require.ErrorIs(t, Append(ctx, tx, 0, mutated), ErrRevisionConflict)
	require.NoError(t, tx.Rollback(ctx))
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 1, count)
	next := proto.Clone(ev).(*eventsv1.SubscriptionStreamEvent)
	next.EventId = uuid.NewString()
	next.AggregateRevision = 2
	next.GetEntitlementChanged().Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_RENEWED
	commitEvent(t, pool, 1, next)
	commitEvent(t, pool, 1, next)
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT aggregate_revision,payload FROM subscription_entitlement_aggregates WHERE aggregate_id=$1", ev.AggregateId).Scan(&revision, &bytes))
	require.EqualValues(t, 2, revision)
	want, err = proto.MarshalOptions{Deterministic: true}.Marshal(next)
	require.NoError(t, err)
	require.Equal(t, want, bytes)
}

func TestConcurrentRevisionCompareAndSetHasOneWinner(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	ev := snapshotEvent()
	commitEvent(t, pool, 0, ev)
	start := make(chan struct{})
	results := make(chan error, 2)
	var ready sync.WaitGroup
	ready.Add(2)
	for i := 0; i < 2; i++ {
		go func() {
			next := proto.Clone(ev).(*eventsv1.SubscriptionStreamEvent)
			next.EventId = uuid.NewString()
			next.AggregateRevision = 2
			tx, err := pool.Begin(ctx)
			ready.Done()
			<-start
			if err == nil {
				err = Append(ctx, tx, 1, next)
				if err == nil {
					err = tx.Commit(ctx)
				}
				_ = tx.Rollback(ctx)
			}
			results <- err
		}()
	}
	ready.Wait()
	close(start)
	first, second := <-results, <-results
	if first == nil {
		require.ErrorIs(t, second, ErrRevisionConflict)
	} else {
		require.ErrorIs(t, first, ErrRevisionConflict)
		require.NoError(t, second)
	}
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Equal(t, 2, count)
}

func TestAppendRejectsInvalidSnapshotWithoutMutation(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	ev := snapshotEvent()
	ev.ProtocolVersion = 2
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	require.Error(t, Append(ctx, tx, 0, ev))
	require.NoError(t, tx.Commit(ctx))
	var count int
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_entitlement_aggregates").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(ctx, "SELECT count(*) FROM subscription_event_outbox").Scan(&count))
	require.Zero(t, count)
}

func TestCrashRedeliveryAndExpiredLeaseCannotComplete(t *testing.T) {
	pool := database(t)
	ctx := context.Background()
	ev := snapshotEvent()
	commitEvent(t, pool, 0, ev)
	s := Store{Pool: pool}
	first, err := s.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	other, err := s.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, other)
	// A process died after PubAck. Only lease time is changed; immutable bytes stay.
	_, err = pool.Exec(ctx, "UPDATE subscription_event_outbox SET lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1", ev.EventId)
	require.NoError(t, err)
	require.ErrorIs(t, s.Complete(ctx, first[0], Ack{Stream: "subscription_events", Sequence: 1}), ErrLeaseLost)
	second, err := s.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, first[0].EventID, second[0].EventID)
	require.Equal(t, first[0].Payload, second[0].Payload)
	require.NotEqual(t, first[0].LeaseToken, second[0].LeaseToken)
	require.ErrorIs(t, s.Complete(ctx, first[0], Ack{Stream: "subscription_events", Sequence: 1}), ErrLeaseLost)
	require.NoError(t, s.Complete(ctx, second[0], Ack{Stream: "subscription_events", Sequence: 1}))
	other, err = s.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Empty(t, other)
	var stream string
	var sequence int64
	require.NoError(t, pool.QueryRow(ctx, "SELECT published_stream, published_sequence FROM subscription_event_outbox WHERE event_id=$1", ev.EventId).Scan(&stream, &sequence))
	require.Equal(t, "subscription_events", stream)
	require.EqualValues(t, 1, sequence)
}

func TestSnapshotValidationRejectsMalformedAuthority(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*eventsv1.SubscriptionStreamEvent)
	}{
		{"version", func(e *eventsv1.SubscriptionStreamEvent) { e.ProtocolVersion = 2 }},
		{"revision", func(e *eventsv1.SubscriptionStreamEvent) { e.AggregateRevision = 0 }},
		{"aggregate", func(e *eventsv1.SubscriptionStreamEvent) { e.AggregateId = uuid.NewString() }},
		{"state", func(e *eventsv1.SubscriptionStreamEvent) { e.GetEntitlementChanged().State = 99 }},
		{"reason", func(e *eventsv1.SubscriptionStreamEvent) { e.GetEntitlementChanged().Reason = 99 }},
		{"deadline", func(e *eventsv1.SubscriptionStreamEvent) {
			e.GetEntitlementChanged().EntitledUntil = e.GetEntitlementChanged().EffectiveAt
		}},
		{"unknown", func(e *eventsv1.SubscriptionStreamEvent) { e.ProtoReflect().SetUnknown([]byte{0xf8, 0x07, 0x01}) }},
		{"missing effective time", func(e *eventsv1.SubscriptionStreamEvent) { e.GetEntitlementChanged().EffectiveAt = nil }},
		{"missing snapshot", func(e *eventsv1.SubscriptionStreamEvent) { e.Payload = nil }},
		{"nil uuid", func(e *eventsv1.SubscriptionStreamEvent) { e.EventId = uuid.Nil.String() }},
	} {
		t.Run(tc.name, func(t *testing.T) { ev := snapshotEvent(); tc.mutate(ev); require.Error(t, Validate(ev)) })
	}
	valid := snapshotEvent()
	require.NoError(t, Validate(valid))
	grace := proto.Clone(valid).(*eventsv1.SubscriptionStreamEvent)
	g := grace.GetEntitlementChanged()
	g.State = eventsv1.EntitlementState_ENTITLEMENT_STATE_GRACE_PERIOD
	g.Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PAYMENT_FAILED
	g.GracePeriodEnd = timestamppb.New(g.EffectiveAt.AsTime().Add(7 * 24 * time.Hour))
	g.EntitledUntil = g.GracePeriodEnd
	require.NoError(t, Validate(grace))
	inactive := proto.Clone(valid).(*eventsv1.SubscriptionStreamEvent)
	i := inactive.GetEntitlementChanged()
	i.State = eventsv1.EntitlementState_ENTITLEMENT_STATE_INACTIVE
	i.Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PERIOD_ENDED
	i.EntitledUntil = i.EffectiveAt
	require.NoError(t, Validate(inactive))
	space := proto.Clone(valid).(*eventsv1.SubscriptionStreamEvent)
	space.AggregateKind = eventsv1.SubscriptionAggregateKind_SUBSCRIPTION_AGGREGATE_KIND_SPACE
	sp := space.GetEntitlementChanged()
	sp.SpaceId = space.AggregateId
	sp.AccountId = ""
	sp.Plan = "space_pro"
	payer := uuid.NewString()
	sp.PurchaserAccountId = &payer
	require.NoError(t, Validate(space))
	sp.PurchaserAccountId = nil
	sp.PurchaserDeleted = true
	sp.Reason = eventsv1.EntitlementReason_ENTITLEMENT_REASON_PURCHASER_PURGED
	require.NoError(t, Validate(space))
}
