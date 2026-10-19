package partysearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	admission "voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/store"
)

func projectionRedis(t *testing.T) *queue.RedisQueue {
	t.Helper()
	addr := os.Getenv("BE011_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("real Redis unavailable: set BE011_TEST_REDIS_ADDR")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	require.NoError(t, rdb.Ping(context.Background()).Err())
	q := &queue.RedisQueue{Client: rdb, Prefix: "rudeus145:projection:" + uuid.NewString() + ":"}
	t.Cleanup(func() {
		ctx := context.Background()
		var cursor uint64
		for {
			keys, next, err := rdb.Scan(ctx, cursor, q.Prefix+"*", 100).Result()
			if err != nil {
				break
			}
			if len(keys) > 0 {
				_ = rdb.Del(ctx, keys...).Err()
			}
			cursor = next
			if cursor == 0 {
				break
			}
		}
		_ = rdb.Close()
	})
	return q
}

func projectionSessions(sessions []store.SearchSession) []queue.SearchRecovery {
	out := make([]queue.SearchRecovery, 0, len(sessions))
	for _, s := range sessions {
		out = append(out, queue.SearchRecovery{SpaceID: s.SpaceID, GameID: s.GameID, Mode: s.Mode, Region: "eu", SessionID: s.ID, ProfileID: s.ProfileID, CreatedAt: s.CreatedAt, Generation: s.RecoveryGeneration})
	}
	return out
}

type projectionSpy struct {
	delegate *queue.RedisQueue
	before   func()
	after    func()
}

func TestPrecutProjectorKeepsFirstAdmissionGateThroughQueueAndAck(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	q := projectionRedis(t)
	done := make(chan error, 1)
	before := func() {
		go func() {
			bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
			defer cancel()
			tx, err := pool.Begin(bounded)
			if err != nil {
				done <- err
				return
			}
			defer tx.Rollback(context.Background())
			if err = admission.AcquireCloseGate(bounded, tx); err == nil {
				var ack bool
				err = tx.QueryRow(bounded, `SELECT projected_at IS NOT NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&ack)
				if err == nil && !ack {
					err = store.ErrSourcePartyUnavailable
				}
			}
			done <- err
		}()
		require.Eventually(t, func() bool {
			select {
			case err := <-done:
				t.Fatalf("first begin bypassed in-flight pre-cut projection: %v", err)
			default:
			}
			var waiting bool
			return pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_catalog.pg_advisory_xact_lock($1)%')`).Scan(&waiting) == nil && waiting
		}, 3*time.Second, 10*time.Millisecond)
	}
	claimed, err := (&Projector{Store: st, Queue: projectionSpy{delegate: q, before: before}}).RunOnce(ctx)
	require.True(t, claimed)
	require.NoError(t, err)
	require.NoError(t, <-done, "first begin must observe the durable ACK after admission guard release")
}

func (s projectionSpy) RecoverParty(ctx context.Context, sessions []queue.SearchRecovery) error {
	if s.before != nil {
		s.before()
	}
	err := s.delegate.RecoverParty(ctx, sessions)
	if s.after != nil {
		s.after()
	}
	return err
}
func (s projectionSpy) RecoverRelease(ctx context.Context, session queue.SearchRecovery) error {
	if s.before != nil {
		s.before()
	}
	err := s.delegate.RecoverRelease(ctx, session)
	if s.after != nil {
		s.after()
	}
	return err
}

func TestPartyProjectorRecoversLostAckWithExactLeaseAndNoDuplicateQueue(t *testing.T) {
	pool := partyDB(t)
	q := projectionRedis(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	first, err := st.ClaimQueueIntent(ctx, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, first)
	busy, err := st.ClaimQueueIntent(ctx, 30*time.Second)
	require.NoError(t, err)
	require.Nil(t, busy)
	cut, err := st.ReadQueueIntent(ctx, *first)
	require.NoError(t, err)
	require.True(t, cut.Enqueue)
	require.Len(t, cut.Sessions, 2)
	require.Zero(t, pool.Stat().AcquiredConns(), "native queue IO must run outside SQL transactions")
	require.NoError(t, q.RecoverParty(ctx, projectionSessions(cut.Sessions)))
	_, err = pool.Exec(ctx, `UPDATE matchmaking_party_search_queue_intents SET lease_until=clock_timestamp()-interval '1 second' WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	claimed, err := (&Projector{Store: st, Queue: projectionSpy{delegate: q, before: func() {
		require.EqualValues(t, 1, pool.Stat().AcquiredConns(), "pre-cut advisory-only guard spans dispatch, without row locks")
	}}}).RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, claimed)
	ids, err := q.ListSessionIDs(ctx, params.GameID, params.Mode, "eu", 10)
	require.NoError(t, err)
	expected := []uuid.UUID{created.Sessions[0].ID, created.Sessions[1].ID}
	require.ElementsMatch(t, expected, ids)
	require.ErrorIs(t, st.FinishQueueIntent(ctx, *first, time.Second, true), store.ErrPartyQueueLeaseLost)
	_, err = st.ReadQueueIntent(ctx, *first)
	require.ErrorIs(t, err, store.ErrPartyQueueLeaseLost)
	var projected, unleased bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT projected_at IS NOT NULL,lease_token IS NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&projected, &unleased))
	require.True(t, projected)
	require.True(t, unleased)
}

func TestPartyProjectorRetriesConflictWithoutPartialProfileLocks(t *testing.T) {
	pool := partyDB(t)
	q := projectionRedis(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	other := uuid.New()
	require.NoError(t, q.AcquireLock(ctx, params.MemberProfileIDs[1], other))
	p := &Projector{Store: st, Queue: q}
	claimed, err := p.RunOnce(ctx)
	require.True(t, claimed)
	require.ErrorIs(t, err, queue.ErrLockHeld)
	depth, err := q.QueueDepth(ctx, params.GameID, params.Mode, "eu")
	require.NoError(t, err)
	require.Zero(t, depth)
	probe := uuid.New()
	require.NoError(t, q.AcquireLock(ctx, params.MemberProfileIDs[0], probe))
	require.NoError(t, q.ReleaseLock(ctx, params.MemberProfileIDs[0], probe))
	var attempts int64
	var projected, unleased bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT attempts,projected_at IS NOT NULL,lease_token IS NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&attempts, &projected, &unleased))
	require.EqualValues(t, 1, attempts)
	require.False(t, projected)
	require.True(t, unleased)
	require.NoError(t, q.ReleaseLock(ctx, params.MemberProfileIDs[1], other))
	_, err = pool.Exec(ctx, `UPDATE matchmaking_party_search_queue_intents SET next_attempt_at=clock_timestamp() WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	claimed, err = p.RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, claimed)
	depth, err = q.QueueDepth(ctx, params.GameID, params.Mode, "eu")
	require.NoError(t, err)
	require.EqualValues(t, 2, depth)
}

func TestPartyProjectorPausedEnqueueCannotOverwriteNativeResetRelease(t *testing.T) {
	pool := partyDB(t)
	q := projectionRedis(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	late := &Projector{Store: st, Queue: projectionSpy{delegate: q, before: func() {
		require.EqualValues(t, 1, pool.Stat().AcquiredConns(), "pre-cut advisory-only guard spans dispatch, without row locks")
		_, err := (&store.MatchStore{Pool: pool}).ResetVoiceParty(ctx, created.Party.ID)
		require.NoError(t, err)
		claimed, err := (&Projector{Store: st, Queue: q}).RunOnce(ctx)
		require.NoError(t, err)
		require.True(t, claimed)
	}}}
	claimed, err := late.RunOnce(ctx)
	require.True(t, claimed)
	require.ErrorIs(t, err, queue.ErrQueueGeneration)
	require.ErrorIs(t, err, store.ErrPartyQueueLeaseLost)
	depth, err := q.QueueDepth(ctx, params.GameID, params.Mode, "eu")
	require.NoError(t, err)
	require.Zero(t, depth)
	var action string
	var revision int64
	var projected bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT action,revision,projected_at IS NOT NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&action, &revision, &projected))
	require.Equal(t, "RELEASE", action)
	require.EqualValues(t, 2, revision)
	require.True(t, projected)
}

func TestPartyProjectorReleasesWholePartyWhenOneSessionStopsSearching(t *testing.T) {
	pool := partyDB(t)
	q := projectionRedis(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	require.NoError(t, q.RecoverParty(ctx, projectionSessions(created.Sessions)))
	_, err = (&store.SessionStore{Pool: pool}).Cancel(ctx, created.Sessions[1].ID)
	require.NoError(t, err)
	// Cleanup must remain possible after private capture erasure. This deliberate
	// privileged mutation is confined to the disposable fixture database.
	_, err = pool.Exec(ctx, `ALTER TABLE parties DISABLE TRIGGER matchmaking_party_source_immutable`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE parties SET voice_member_authority_version=NULL,voice_member_authority=NULL,voice_member_authority_sha256=NULL,voice_source_authority_sha256=NULL WHERE id=$1`, created.Party.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `ALTER TABLE parties ENABLE TRIGGER matchmaking_party_source_immutable`)
	require.NoError(t, err)
	claimed, err := (&Projector{Store: st, Queue: q}).RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, claimed)
	depth, err := q.QueueDepth(ctx, params.GameID, params.Mode, "eu")
	require.NoError(t, err)
	require.Zero(t, depth)
	for _, s := range created.Sessions {
		probe := uuid.New()
		require.NoError(t, q.AcquireLock(ctx, s.ProfileID, probe))
		require.NoError(t, q.ReleaseLock(ctx, s.ProfileID, probe))
	}
}

func TestPartyProjectorDoesNotAckStaleSessionCutWhenIntentRevisionIsUnchanged(t *testing.T) {
	pool := partyDB(t)
	q := projectionRedis(t)
	ctx := context.Background()
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	late := &Projector{Store: st, Queue: projectionSpy{delegate: q, before: func() {
		require.EqualValues(t, 1, pool.Stat().AcquiredConns(), "pre-cut advisory-only guard spans dispatch, without row locks")
		_, err := (&store.SessionStore{Pool: pool}).Cancel(ctx, created.Sessions[1].ID)
		require.NoError(t, err)
	}}}
	claimed, err := late.RunOnce(ctx)
	require.True(t, claimed)
	require.ErrorIs(t, err, store.ErrPartyQueueProjectionChanged)
	var projected bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT projected_at IS NOT NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&projected))
	require.False(t, projected)
	_, err = pool.Exec(ctx, `UPDATE matchmaking_party_search_queue_intents SET next_attempt_at=clock_timestamp() WHERE party_id=$1`, created.Party.ID)
	require.NoError(t, err)
	claimed, err = (&Projector{Store: st, Queue: q}).RunOnce(ctx)
	require.NoError(t, err)
	require.True(t, claimed)
	depth, err := q.QueueDepth(ctx, params.GameID, params.Mode, "eu")
	require.NoError(t, err)
	require.Zero(t, depth)
}

func TestPartyProjectorFinalAckUsesFreshCutAfterWaitingForSessionWriter(t *testing.T) {
	pool := partyDB(t)
	q := projectionRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	params := partyParams(t, pool)
	st := &store.PartyStore{Pool: pool}
	created, err := st.CreateSourceParty(ctx, params)
	require.NoError(t, err)
	lease, err := st.ClaimQueueIntent(ctx, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lease)
	cut, err := st.ReadQueueIntent(ctx, *lease)
	require.NoError(t, err)
	require.NoError(t, q.RecoverParty(ctx, projectionSessions(cut.Sessions)))
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback(context.Background()) }()
	_, err = writer.Exec(ctx, `UPDATE search_sessions SET status='cancelled',recovery_generation=recovery_generation+1,updated_at=clock_timestamp() WHERE id=$1`, created.Sessions[1].ID)
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() { finished <- st.FinishQueueIntent(ctx, cut.Lease, 10*time.Second, true) }()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid()
AND wait_event_type='Lock' AND query LIKE '%SELECT id FROM search_sessions WHERE party_id=%FOR UPDATE%')`).Scan(&waiting)
		return err == nil && waiting
	}, 3*time.Second, 10*time.Millisecond, "final acknowledgment must wait for the actual session writer")
	require.NoError(t, writer.Commit(ctx))
	require.ErrorIs(t, <-finished, store.ErrPartyQueueProjectionChanged)
	var projected bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT projected_at IS NOT NULL FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, created.Party.ID).Scan(&projected))
	require.False(t, projected)
}
