package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeCancelClosedWholePartyAndExactRetry(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	st := &store.MatchStore{Pool: pool, NativeAdmission: &binding}
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	var actor = p.MemberAuthorities[1]
	var session uuid.UUID
	for _, s := range party.Sessions {
		if s.ProfileID == actor.ProfileID {
			session = s.ID
		}
	}
	require.NotEqual(t, uuid.Nil, session)
	require.Error(t, st.CancelNativeSearch(ctx, uuid.New(), actor.ProfileID, session), "knowing profile/session is insufficient account authority")
	require.NoError(t, st.CancelNativeSearch(ctx, actor.AccountID, actor.ProfileID, session))
	var cancelled, outbox int
	var action string
	var revision int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='cancelled'`, party.Party.ID).Scan(&cancelled))
	require.Equal(t, len(p.MemberProfileIDs), cancelled)
	require.NoError(t, pool.QueryRow(ctx, `SELECT action,revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&action, &revision))
	require.Equal(t, "RELEASE", action)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&outbox))
	require.Equal(t, len(p.MemberProfileIDs), outbox)
	require.NoError(t, st.CancelNativeSearch(ctx, actor.AccountID, actor.ProfileID, session))
	var after int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&after))
	require.Equal(t, revision, after)
}

func TestNativeCancelRollsBackWholePartyWhenCancellationEvidenceFails(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	binding, _, _ := historyReceiptFixture(t, pool)
	p := partyParams(t, pool)
	party, err := (&store.PartyStore{Pool: pool, NativeAdmission: &binding}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION fail_native_cancel() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected cancel failure'; END $$;CREATE TRIGGER fail_native_cancel BEFORE INSERT ON matchmaking_voice_roster_cancellation_outbox FOR EACH ROW EXECUTE FUNCTION fail_native_cancel()`)
	require.NoError(t, err)
	var session uuid.UUID
	for _, row := range party.Sessions {
		if row.ProfileID == p.InitiatorProfileID {
			session = row.ID
		}
	}
	require.NotEqual(t, uuid.Nil, session)
	err = (&store.MatchStore{Pool: pool, NativeAdmission: &binding}).CancelNativeSearch(ctx, p.InitiatorAccountID, p.InitiatorProfileID, session)
	require.ErrorContains(t, err, "injected cancel failure")
	var searching int
	var action string
	var disbanded bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='searching'`, party.Party.ID).Scan(&searching))
	require.Equal(t, len(p.MemberProfileIDs), searching)
	require.NoError(t, pool.QueryRow(ctx, `SELECT disbanded_at IS NOT NULL FROM parties WHERE id=$1`, party.Party.ID).Scan(&disbanded))
	require.False(t, disbanded)
	require.NoError(t, pool.QueryRow(ctx, `SELECT action FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, party.Party.ID).Scan(&action))
	require.Equal(t, "ENQUEUE", action)
}

func TestNativeCancelNeverReinterpretsPendingMatchAsSearch(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	var session uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id FROM search_sessions WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, p.InitiatorProfileID).Scan(&session))
	require.ErrorIs(t, st.CancelNativeSearch(ctx, p.InitiatorAccountID, p.InitiatorProfileID, session), store.ErrSessionNotSearchable)
	m, err := st.Get(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusPendingAccept, m.Status)
	var outbox int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&outbox))
	require.Zero(t, outbox)
}

func TestNativeCancelRechecksReservationAfterActualPartyLockWait(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	var session, party uuid.UUID
	require.NoError(t, pool.QueryRow(ctx, `SELECT id,party_id FROM search_sessions WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, p.InitiatorProfileID).Scan(&session, &party))
	_, err := pool.Exec(ctx, `UPDATE search_sessions SET status='searching',match_id=NULL,matched_at=NULL WHERE party_id=$1`, party)
	require.NoError(t, err)
	holder, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer holder.Rollback(ctx)
	_, err = holder.Exec(ctx, `SELECT id FROM parties WHERE id=$1 FOR UPDATE`, party)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		done <- st.CancelNativeSearch(bounded, p.InitiatorAccountID, p.InitiatorProfileID, session)
	}()
	require.Eventually(t, func() bool {
		select {
		case e := <-done:
			t.Fatalf("cancel bypassed party lock: %v", e)
		default:
		}
		var waiting bool
		return pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT id FROM parties WHERE id=ANY%')`).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	// This controlled fixture models a complete reservation committing while
	// cancellation waits. It is not a source-admission/producer qualification.
	_, err = holder.Exec(ctx, `UPDATE search_sessions SET status='pending_accept',match_id=$2 WHERE party_id=$1`, party, intent.MatchID)
	require.NoError(t, err)
	require.NoError(t, holder.Commit(ctx))
	require.ErrorIs(t, <-done, store.ErrSessionNotSearchable)
	var cancelled, outbox int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM search_sessions WHERE party_id=$1 AND status='cancelled'`, party).Scan(&cancelled))
	require.Zero(t, cancelled)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_voice_roster_cancellation_outbox`).Scan(&outbox))
	require.Zero(t, outbox)
	m, err := st.Get(ctx, intent.MatchID)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusPendingAccept, m.Status)
}
