package providerlifecycle

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/subscription/internal/gracereminders"
)

func TestGraceTransitionAtomicallySchedulesBothPayerKinds(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	st := Store{Pool: pool}
	for _, kind := range []eventsv1.SubscriptionAggregateKind{1, 2, 2} {
		c := fakeCommand(kind)
		c.EffectiveAt = time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
		c.PeriodStart = c.EffectiveAt
		c.PeriodEnd = c.EffectiveAt.AddDate(0, 1, 0)
		_, err := st.Apply(ctx, c)
		require.NoError(t, err)
		failure := nextCommand(c, failed, c.EffectiveAt.Add(time.Minute))
		grace, err := st.Apply(ctx, failure)
		require.NoError(t, err)
		var exists bool
		require.NoError(t, pool.QueryRow(ctx, "SELECT to_regclass('subscription_grace_reminders') IS NOT NULL").Scan(&exists))
		require.True(t, exists, "committed grace must atomically schedule revision-bound D1/D3/D7 reminders")
		var count int
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_grace_reminders WHERE aggregate_kind=$1 AND aggregate_id=$2 AND grace_revision=$3`, int16(kind), c.AggregateID, int64(grace.Event.AggregateRevision)).Scan(&count))
		require.Equal(t, 3, count)
		replay, err := st.Apply(ctx, failure)
		require.NoError(t, err)
		require.Equal(t, grace.Event.EventId, replay.Event.EventId)
		recover := nextCommand(failure, recovered, failure.EffectiveAt.Add(time.Minute))
		_, err = st.Apply(ctx, recover)
		require.NoError(t, err)
		require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_grace_reminders WHERE aggregate_kind=$1 AND aggregate_id=$2 AND status='SUPPRESSED'`, int16(kind), c.AggregateID).Scan(&count))
		require.Equal(t, 3, count, "recovery cancels every undelivered reminder")
	}
}

func graceFixture(t *testing.T, pool *pgxpool.Pool, age time.Duration, payer string) (Command, Result) {
	t.Helper()
	c := fakeCommand(2)
	if payer != "" {
		c.PurchaserID = payer
	}
	c.EffectiveAt = time.Now().UTC().Add(-age - time.Hour).Truncate(time.Microsecond)
	c.PeriodStart = c.EffectiveAt
	c.PeriodEnd = c.EffectiveAt.AddDate(0, 1, 0)
	st := Store{Pool: pool}
	_, err := st.Apply(context.Background(), c)
	require.NoError(t, err)
	failure := nextCommand(c, failed, c.EffectiveAt.Add(time.Hour))
	result, err := st.Apply(context.Background(), failure)
	require.NoError(t, err)
	return failure, result
}

func reminderJS(t *testing.T) nats.JetStreamContext {
	t.Helper()
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "subscription_events", Subjects: []string{gracereminders.Subject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	return js
}

// Real scheduling ages exercise DB time without a production test clock.
func TestReminderWindowsConcurrentClaimsAndSpacePayerWire(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	payer := uuid.NewString()
	expected := map[string]int32{}
	for i, age := range []time.Duration{time.Hour, 3 * 24 * time.Hour, 6*24*time.Hour + time.Hour} {
		failure, grace := graceFixture(t, pool, age, payer)
		expected[failure.AggregateID] = []int32{1, 3, 7}[i]
		require.Equal(t, grace.Event.GetEntitlementChanged().GracePeriodEnd.AsTime(), failure.EffectiveAt.Add(7*24*time.Hour), "all Pro benefits retain their P7D entitlement boundary")
		rows, err := pool.Query(ctx, `SELECT day,due_at,window_end FROM subscription_grace_reminders WHERE aggregate_id=$1 ORDER BY day`, failure.AggregateID)
		require.NoError(t, err)
		defer rows.Close()
		for rows.Next() {
			var day int32
			var due, end time.Time
			require.NoError(t, rows.Scan(&day, &due, &end))
			require.Equal(t, failure.EffectiveAt.Add(time.Duration(day-1)*24*time.Hour), due.UTC())
			delta := map[int32]time.Duration{1: 2 * 24 * time.Hour, 3: 6 * 24 * time.Hour, 7: 7 * 24 * time.Hour}[day]
			require.Equal(t, failure.EffectiveAt.Add(delta), end.UTC())
		}
		require.NoError(t, rows.Err())
		rows.Close()
	}
	st := gracereminders.Store{Pool: pool}
	var wg sync.WaitGroup
	batches := make(chan []gracereminders.Delivery, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); d, e := st.Claim(ctx, 100, time.Minute); batches <- d; errs <- e }()
	}
	wg.Wait()
	require.NoError(t, <-errs)
	require.NoError(t, <-errs)
	all := append(<-batches, <-batches...)
	require.Len(t, all, 3, "one active reminder window per Space despite concurrent replicas")
	js := reminderJS(t)
	seen := map[string]bool{}
	for _, d := range all {
		require.False(t, seen[d.EventID])
		seen[d.EventID] = true
		require.NoError(t, st.Dispatch(ctx, js, d))
		env := new(eventsv1.SubscriptionStreamEvent)
		require.NoError(t, proto.Unmarshal(d.Payload, env))
		require.EqualValues(t, 1, env.ProtocolVersion)
		require.EqualValues(t, 2, env.AggregateKind)
		require.EqualValues(t, 2, env.AggregateRevision)
		p := env.GetGraceReminder()
		require.NotNil(t, p)
		require.Equal(t, payer, p.AccountId)
		require.Equal(t, "space_pro", p.Plan)
		require.Equal(t, env.AggregateId, p.GetSpaceId())
		require.Equal(t, expected[env.AggregateId], p.Day)
		require.ErrorIs(t, st.Dispatch(ctx, js, d), gracereminders.ErrLeaseLost)
	}
	info, err := js.StreamInfo("subscription_events")
	require.NoError(t, err)
	require.EqualValues(t, 3, info.State.Msgs)
	for seq := uint64(1); seq <= 3; seq++ {
		msg, err := js.GetMsg("subscription_events", seq)
		require.NoError(t, err)
		env := new(eventsv1.SubscriptionStreamEvent)
		require.NoError(t, proto.Unmarshal(msg.Data, env))
		require.Equal(t, env.EventId, msg.Header.Get(nats.MsgIdHdr))
	}
	var expired int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_grace_reminders WHERE status='EXPIRED'`).Scan(&expired))
	require.Equal(t, 3, expired, "missed D1/D3 windows retained instead of burst")
}

type lostReminderAck struct{ nats.JetStreamContext }

func (js lostReminderAck) PublishMsg(msg *nats.Msg, opts ...nats.PubOpt) (*nats.PubAck, error) {
	_, err := js.JetStreamContext.PublishMsg(msg, opts...)
	if err != nil {
		return nil, err
	}
	return nil, errors.New("injected lost PubAck")
}

func TestReminderLostAckReclaimKeepsBytesAndFencesOldLease(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	graceFixture(t, pool, time.Hour, "")
	st := gracereminders.Store{Pool: pool}
	js := reminderJS(t)
	first, err := st.Claim(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Error(t, st.Dispatch(ctx, lostReminderAck{js}, first[0]))
	_, err = pool.Exec(ctx, `UPDATE subscription_grace_reminders SET lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1`, first[0].EventID)
	require.NoError(t, err)
	second, err := st.Claim(ctx, 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, second, 1)
	require.Equal(t, first[0].EventID, second[0].EventID)
	require.Equal(t, first[0].Payload, second[0].Payload)
	require.NotEqual(t, first[0].LeaseToken, second[0].LeaseToken)
	require.ErrorIs(t, st.Dispatch(ctx, js, first[0]), gracereminders.ErrLeaseLost)
	require.ErrorIs(t, st.Retry(ctx, first[0]), gracereminders.ErrLeaseLost)
	require.NoError(t, st.Dispatch(ctx, js, second[0]))
	info, err := js.StreamInfo("subscription_events")
	require.NoError(t, err)
	require.EqualValues(t, 1, info.State.Msgs, "resend uses stable Nats-Msg-Id")
}

func TestReminderRecoveryAndExpirySuppressClaimedRows(t *testing.T) {
	for _, reason := range []eventsv1.EntitlementReason{recovered, expired} {
		t.Run(reason.String(), func(t *testing.T) {
			pool := testDB(t)
			ctx := context.Background()
			age := time.Hour
			if reason == expired {
				age = 8 * 24 * time.Hour
			}
			failure, _ := graceFixture(t, pool, age, "")
			st := gracereminders.Store{Pool: pool}
			var claimed []gracereminders.Delivery
			if reason == recovered {
				var err error
				claimed, err = st.Claim(ctx, 10, time.Minute)
				require.NoError(t, err)
				require.Len(t, claimed, 1)
			}
			at := failure.EffectiveAt.Add(time.Minute)
			if reason == expired {
				at = failure.EffectiveAt.Add(7 * 24 * time.Hour)
			}
			_, err := (Store{Pool: pool}).Apply(ctx, nextCommand(failure, reason, at))
			require.NoError(t, err)
			var count int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_grace_reminders WHERE status='SUPPRESSED'`).Scan(&count))
			require.Equal(t, 3, count)
			if len(claimed) > 0 {
				require.ErrorIs(t, st.Dispatch(ctx, nil, claimed[0]), gracereminders.ErrLeaseLost)
			}
			due, err := st.Claim(ctx, 10, time.Minute)
			require.NoError(t, err)
			require.Empty(t, due)
		})
	}
}

func TestReminderFailureRollsBackTransitionAndMigrationReapplies(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	st := Store{Pool: pool}
	c := fakeCommand(2)
	_, err := st.Apply(ctx, c)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE subscription_grace_reminders ADD CONSTRAINT injected_reminder_failure CHECK(false)`)
	require.NoError(t, err)
	failure := nextCommand(c, failed, c.PeriodEnd)
	_, err = st.Apply(ctx, failure)
	require.Error(t, err)
	var revision, count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT aggregate_revision FROM subscription_entitlement_aggregates`).Scan(&revision))
	require.Equal(t, 1, revision)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM subscription_provider_outcomes`).Scan(&count))
	require.Equal(t, 1, count)
	_, err = pool.Exec(ctx, `ALTER TABLE subscription_grace_reminders DROP CONSTRAINT injected_reminder_failure`)
	require.NoError(t, err)
	_, err = st.Apply(ctx, failure)
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, direction := range []string{"down", "up"} {
		body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/subscription_db/000006_grace_reminder_outbox."+direction+".sql"))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err)
	}
}

// The worker first locks its reminder, then may wait on a canonical aggregate
// row held by a different database transaction. Real DB time must be checked
// after that wait, for both the exclusive delivery window and the lease.
func TestReminderDispatchRechecksWindowAndLeaseAfterAuthorityWait(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	for _, boundary := range []string{"window", "lease"} {
		t.Run(boundary, func(t *testing.T) {
			age := time.Hour
			if boundary == "window" {
				age = 2*24*time.Hour - 2*time.Second
			}
			failure, _ := graceFixture(t, pool, age, "")
			st := gracereminders.Store{Pool: pool}
			claimed, err := st.Claim(ctx, 100, time.Minute)
			require.NoError(t, err)
			var d gracereminders.Delivery
			for _, item := range claimed {
				e := new(eventsv1.SubscriptionStreamEvent)
				require.NoError(t, proto.Unmarshal(item.Payload, e))
				if e.AggregateId == failure.AggregateID {
					d = item
				}
			}
			require.NotEmpty(t, d.EventID)
			var deadline time.Time
			if boundary == "lease" {
				err = pool.QueryRow(ctx, `UPDATE subscription_grace_reminders SET lease_until=clock_timestamp()+interval '2 seconds' WHERE event_id=$1 RETURNING lease_until`, d.EventID).Scan(&deadline)
			} else {
				err = pool.QueryRow(ctx, `SELECT window_end FROM subscription_grace_reminders WHERE event_id=$1`, d.EventID).Scan(&deadline)
			}
			require.NoError(t, err)
			holder, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer func() { _ = holder.Rollback(ctx) }()
			var holderPID int
			require.NoError(t, holder.QueryRow(ctx, `SELECT pg_backend_pid() FROM subscription_entitlement_aggregates WHERE aggregate_kind=2 AND aggregate_id=$1 FOR UPDATE`, failure.AggregateID).Scan(&holderPID))
			js := reminderJS(t)
			result := make(chan error, 1)
			go func() { result <- st.Dispatch(ctx, js, d) }()
			require.Eventually(t, func() bool {
				var blocked bool
				e := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1=ANY(pg_blocking_pids(pid)) AND query LIKE '%SELECT aggregate_revision,payload,payload_hash%')`, holderPID).Scan(&blocked)
				return e == nil && blocked
			}, time.Second, 10*time.Millisecond, "Dispatch reached canonical row wait after its initial reminder validity read")
			require.Eventually(t, func() bool {
				var crossed bool
				e := pool.QueryRow(ctx, `SELECT clock_timestamp()>=$1`, deadline).Scan(&crossed)
				return e == nil && crossed
			}, 3*time.Second, 10*time.Millisecond, "real PostgreSQL deadline crosses while canonical row is held")
			require.NoError(t, holder.Commit(ctx))
			select {
			case err = <-result:
			case <-time.After(3 * time.Second):
				t.Fatal("Dispatch did not settle after releasing authority row")
			}
			if boundary == "lease" {
				require.ErrorIs(t, err, gracereminders.ErrLeaseLost)
			} else {
				require.NoError(t, err)
				var status string
				require.NoError(t, pool.QueryRow(ctx, `SELECT status FROM subscription_grace_reminders WHERE event_id=$1`, d.EventID).Scan(&status))
				require.Equal(t, "EXPIRED", status)
			}
			info, err := js.StreamInfo("subscription_events")
			require.NoError(t, err)
			require.Zero(t, info.State.Msgs, "crossing a window or lease while waiting must not publish")
		})
	}
}
