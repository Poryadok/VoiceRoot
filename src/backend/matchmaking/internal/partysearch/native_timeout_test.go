package partysearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/mmevents"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/queue"
	"voice/backend/matchmaking/internal/runtimeconfig"
	"voice/backend/matchmaking/internal/store"
	"voice/backend/matchmaking/internal/timeout"
)

func TestNativeTimeoutSweeperCommitsPartyReleaseAndEveryDurableFact(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE search_sessions SET timeout_at=statement_timestamp()-interval '1 second' WHERE party_id=$1`, party.Party.ID)
	require.NoError(t, err)
	addr := os.Getenv("BE011_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("owned native Redis unavailable")
	}
	redis := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = redis.Close() })
	q := &queue.RedisQueue{Client: redis, Prefix: "native-timeout-" + party.Party.ID.String()}
	worker := &timeout.Sweeper{Sessions: &store.SessionStore{Pool: pool}, Matches: &store.MatchStore{Pool: pool, NativeAdmission: &binding}, Queue: q, Events: mmevents.NoopPublisher{}}
	require.NoError(t, worker.RunNativeOnce(ctx))
	var count, facts, cancellations int
	var action string
	var disbanded bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='timeout' AND recovery_generation=1`, party.Party.ID).Scan(&count))
	require.Equal(t, len(p.MemberProfileIDs), count)
	require.NoError(t, pool.QueryRow(ctx, `SELECT action FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&action))
	require.Equal(t, "RELEASE", action)
	require.NoError(t, pool.QueryRow(ctx, `SELECT disbanded_at IS NOT NULL FROM parties WHERE id=$1`, party.Party.ID).Scan(&disbanded))
	require.True(t, disbanded)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox WHERE party_id=$1`, party.Party.ID).Scan(&facts))
	require.Equal(t, len(p.MemberProfileIDs), facts)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&cancellations))
	require.Zero(t, cancellations)
	// A restarted sweeper cannot relabel facts or advance the terminal revision.
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&revision))
	require.NoError(t, worker.RunNativeOnce(ctx))
	var after int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&after))
	require.Equal(t, revision, after)
}

func nativeTimeoutParty(t *testing.T) (*pgxpool.Pool, *store.MatchStore, store.SourcePartySearch, store.CreateSourcePartyParams) {
	t.Helper()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(context.Background(), p)
	require.NoError(t, err)
	return pool, &store.MatchStore{Pool: pool, NativeAdmission: &binding}, party, p
}

func dueNativeTimeoutParty(t *testing.T, pool *pgxpool.Pool, party uuid.UUID) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `UPDATE search_sessions SET timeout_at=statement_timestamp()-interval '1 minute' WHERE party_id=$1`, party)
	require.NoError(t, err)
}

func TestNativeTimeoutClosedWholePartyConcurrentAndLostAckReplay(t *testing.T) {
	ctx := context.Background()
	pool, st, party, p := nativeTimeoutParty(t)
	dueNativeTimeoutParty(t, pool, party.Party.ID)
	_, err := pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	type outcome struct {
		result store.NativeTimeoutResult
		err    error
	}
	done := make(chan outcome, 2)
	start := make(chan struct{})
	for _, session := range party.Sessions {
		id := session.ID
		go func() { <-start; result, err := st.ExpireNativeSearchingParty(ctx, id); done <- outcome{result, err} }()
	}
	close(start)
	expired, replayed := 0, 0
	for range 2 {
		result := <-done
		require.NoError(t, result.err)
		if result.result.Expired {
			expired++
		}
		if result.result.Replay {
			replayed++
		}
	}
	require.Equal(t, 1, expired)
	require.Equal(t, 1, replayed)
	result, err := st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
	require.NoError(t, err)
	require.True(t, result.Replay)
	var facts int
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox WHERE party_id=$1`, party.Party.ID).Scan(&facts))
	require.Equal(t, len(p.MemberProfileIDs), facts)
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&revision))
	require.EqualValues(t, 2, revision)
	require.ErrorIs(t, st.CancelNativeSearch(ctx, p.InitiatorAccountID, p.InitiatorProfileID, party.Sessions[0].ID), store.ErrSessionNotSearchable)
	var cancelled int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&cancelled))
	require.Zero(t, cancelled)
}

func TestNativeTimeoutEvidenceFailureRollsBackEveryMemberAndIntent(t *testing.T) {
	ctx := context.Background()
	pool, st, party, _ := nativeTimeoutParty(t)
	dueNativeTimeoutParty(t, pool, party.Party.ID)
	_, err := pool.Exec(ctx, `CREATE FUNCTION fail_native_timeout() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected timeout fact failure'; END $$; CREATE TRIGGER fail_native_timeout BEFORE INSERT ON matchmaking_native_timeout_outbox FOR EACH ROW EXECUTE FUNCTION fail_native_timeout()`)
	require.NoError(t, err)
	_, err = st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
	require.ErrorContains(t, err, "injected timeout fact failure")
	var searching, facts int
	var revision int64
	var action string
	var disbanded bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching' AND recovery_generation=0`, party.Party.ID).Scan(&searching))
	require.Equal(t, len(party.Sessions), searching)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox`).Scan(&facts))
	require.Zero(t, facts)
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision,action FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&revision, &action))
	require.EqualValues(t, 1, revision)
	require.Equal(t, "ENQUEUE", action)
	require.NoError(t, pool.QueryRow(ctx, `SELECT disbanded_at IS NOT NULL FROM parties WHERE id=$1`, party.Party.ID).Scan(&disbanded))
	require.False(t, disbanded)
}

func TestNativeTimeoutRefusesIncompleteOrMixedEvidenceWithoutRepair(t *testing.T) {
	for _, kind := range []string{"null_deadline", "mixed_deadline", "changed_member", "missing_intent", "missing_origin", "wrong_binding", "missing_binding"} {
		t.Run(kind, func(t *testing.T) {
			ctx := context.Background()
			var pool *pgxpool.Pool
			var st *store.MatchStore
			var party store.SourcePartySearch
			if kind == "missing_origin" {
				pool = partyDB(t)
				p := partyParams(t, pool)
				var err error
				party, err = (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
				require.NoError(t, err, "actual historical typed capture before installation, never enrolled")
				binding, _, _ := historyReceiptFixture(t, pool)
				st = &store.MatchStore{Pool: pool, NativeAdmission: &binding}
			} else {
				pool, st, party, _ = nativeTimeoutParty(t)
			}
			dueNativeTimeoutParty(t, pool, party.Party.ID)
			var err error
			switch kind {
			case "null_deadline":
				_, err = pool.Exec(ctx, `UPDATE search_sessions SET timeout_at=NULL WHERE id=$1`, party.Sessions[0].ID)
			case "mixed_deadline":
				_, err = pool.Exec(ctx, `UPDATE search_sessions SET timeout_at=timeout_at+interval '1 second' WHERE id=$1`, party.Sessions[0].ID)
			case "changed_member":
				_, err = pool.Exec(ctx, `UPDATE search_sessions SET profile_id=$2 WHERE id=$1`, party.Sessions[0].ID, uuid.New())
			case "missing_intent":
				_, err = pool.Exec(ctx, `DELETE FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID)
			case "missing_origin": // deliberately historical capture above; no retained evidence is deleted
			case "wrong_binding":
				copy := *st.NativeAdmission
				copy.TransitionID = uuid.New()
				st.NativeAdmission = &copy
			case "missing_binding":
				st.NativeAdmission = nil
			}
			require.NoError(t, err)
			_, err = st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
			require.Error(t, err)
			var searching, facts int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching' AND recovery_generation=0`, party.Party.ID).Scan(&searching))
			require.Equal(t, len(party.Sessions), searching)
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox`).Scan(&facts))
			require.Zero(t, facts)
		})
	}
}

func TestNativeTimeoutUsesDatabaseDeadlineAndInvalidatesOldQueueAck(t *testing.T) {
	ctx := context.Background()
	pool, st, party, p := nativeTimeoutParty(t)
	parties := &store.PartyStore{Pool: pool, NativeAdmission: st.NativeAdmission}
	lease, err := parties.ClaimQueueIntent(ctx, 30*time.Second)
	require.NoError(t, err)
	require.NotNil(t, lease)
	cut, err := parties.ReadQueueIntent(ctx, *lease)
	require.NoError(t, err)
	validator := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	projector := &Projector{Store: parties, Validator: validator, ForwardLifetime: &projectionLifetimeFixture{}, Resetter: st}
	release, err := projector.authorizeNativeProjection(ctx, &cut)
	require.NoError(t, err)
	defer release()
	result, err := st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
	require.NoError(t, err)
	require.False(t, result.Expired)
	require.False(t, result.Replay)
	dueNativeTimeoutParty(t, pool, party.Party.ID)
	result, err = st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
	require.NoError(t, err)
	require.True(t, result.Expired)
	require.ErrorIs(t, parties.FinishQueueIntent(ctx, cut.Lease, time.Second, true), store.ErrPartyQueueLeaseLost)
}

func TestNativeTimeoutRechecksPendingReservationAfterActualPartyWait(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	var partyID, sessionID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT party_id,id FROM search_sessions WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, p.InitiatorProfileID).Scan(&partyID, &sessionID))
	_, err := pool.Exec(ctx, `UPDATE search_sessions SET status='searching',match_id=NULL,timeout_at=statement_timestamp()-interval '1 minute' WHERE party_id=$1`, partyID)
	require.NoError(t, err)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(ctx)
	_, err = holder.Exec(ctx, `SELECT id FROM parties WHERE id=$1 FOR UPDATE`, partyID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := st.ExpireNativeSearchingParty(bounded, sessionID)
		done <- err
	}()
	require.Eventually(t, func() bool {
		select {
		case err := <-done:
			t.Fatalf("timeout bypassed held party: %v", err)
		default:
		}
		var waiting bool
		return pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id,leader_profile_id,member_profile_ids%FOR UPDATE')`).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	// Controlled reservation scheduling fixture, not a producer qualification.
	_, err = holder.Exec(ctx, `UPDATE search_sessions SET status='pending_accept',match_id=$2 WHERE party_id=$1`, partyID, intent.MatchID)
	require.NoError(t, err)
	require.NoError(t, holder.Commit(ctx))
	require.ErrorIs(t, <-done, store.ErrSessionNotSearchable)
	var facts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox`).Scan(&facts))
	require.Zero(t, facts)
	match, err := st.Get(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusPendingAccept, match.Status)
}

func TestNativeTimeoutUsesDbClockAfterWaitAndIgnoresAheadScanner(t *testing.T) {
	ctx := context.Background()
	pool, st, party, _ := nativeTimeoutParty(t)
	worker := &timeout.Sweeper{Sessions: &store.SessionStore{Pool: pool}, Matches: st, Events: mmevents.NoopPublisher{}, Timing: runtimeconfig.SearchTiming{NudgeAfter: 24 * time.Hour}, Now: func() time.Time { return time.Now().Add(2 * time.Hour) }}
	require.NoError(t, worker.RunNativeOnce(ctx), "a scanner clock ahead cannot authorize the stored future deadline")
	var searching int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching' AND recovery_generation=0`, party.Party.ID).Scan(&searching))
	require.Equal(t, len(party.Sessions), searching)
	_, err := pool.Exec(ctx, `UPDATE search_sessions SET timeout_at=statement_timestamp()+interval '1 second' WHERE party_id=$1`, party.Party.ID)
	require.NoError(t, err)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(ctx)
	_, err = holder.Exec(ctx, `SELECT id FROM parties WHERE id=$1 FOR UPDATE`, party.Party.ID)
	require.NoError(t, err)
	done := make(chan store.NativeTimeoutResult, 1)
	failed := make(chan error, 1)
	go func() {
		result, err := st.ExpireNativeSearchingParty(ctx, party.Sessions[0].ID)
		if err != nil {
			failed <- err
			return
		}
		done <- result
	}()
	require.Eventually(t, func() bool {
		var wait bool
		return pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id,leader_profile_id,member_profile_ids%FOR UPDATE')`).Scan(&wait) == nil && wait
	}, time.Second, 10*time.Millisecond)
	var future bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT timeout_at>clock_timestamp() FROM search_sessions WHERE id=$1`, party.Sessions[0].ID).Scan(&future))
	require.True(t, future)
	require.Eventually(t, func() bool {
		var due bool
		return pool.QueryRow(ctx, `SELECT timeout_at<=clock_timestamp() FROM search_sessions WHERE id=$1`, party.Sessions[0].ID).Scan(&due) == nil && due
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, holder.Commit(ctx))
	select {
	case err := <-failed:
		t.Fatal(err)
	case result := <-done:
		require.True(t, result.Expired)
	case <-time.After(3 * time.Second):
		t.Fatal("expiry did not finish after held lock")
	}
}

func TestNativeTimeoutPreservesCancellationWinner(t *testing.T) {
	ctx := context.Background()
	pool, st, party, p := nativeTimeoutParty(t)
	dueNativeTimeoutParty(t, pool, party.Party.ID)
	var session uuid.UUID
	for _, row := range party.Sessions {
		if row.ProfileID == p.InitiatorProfileID {
			session = row.ID
		}
	}
	require.NoError(t, st.CancelNativeSearch(ctx, p.InitiatorAccountID, p.InitiatorProfileID, session))
	_, err := st.ExpireNativeSearchingParty(ctx, session)
	require.ErrorIs(t, err, store.ErrSessionNotSearchable)
	var cancelled, timeouts int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='cancelled' AND recovery_generation=1`, party.Party.ID).Scan(&cancelled))
	require.Equal(t, len(party.Sessions), cancelled)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_native_timeout_outbox`).Scan(&timeouts))
	require.Zero(t, timeouts)
}
