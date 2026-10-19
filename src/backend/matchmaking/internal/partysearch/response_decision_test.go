package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/mmadmission"
	"voice/backend/matchmaking/internal/partysource"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeResponseWaitsForCloserAndRefusesFreshClosedHead(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	params := store.ResponseParams{MatchID: intent.MatchID, AccountID: p.MemberAuthorities[0].AccountID, ProfileID: p.MemberAuthorities[0].ProfileID, OperationID: uuid.New(), SessionEpoch: 99, RequestHash: [32]byte{1}, Accept: true}
	candidate, proof := freshResponseProof(t, pool, st, p, params)
	closer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer closer.Rollback(ctx)
	require.NoError(t, mmadmission.AcquireCloseGate(ctx, closer))
	_, err = closer.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		_, e := st.RecordValidatedResponse(bounded, candidate, proof)
		done <- e
	}()
	var pid int
	require.Eventually(t, func() bool {
		select {
		case e := <-done:
			t.Fatalf("response bypassed FIRST gate: %v", e)
		default:
		}
		return pool.QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%pg_catalog.pg_advisory_xact_lock_shared%' LIMIT 1`).Scan(&pid) == nil
	}, 3*time.Second, 10*time.Millisecond)
	var granted int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE pid=$1 AND locktype='advisory' AND granted`, pid).Scan(&granted))
	require.Zero(t, granted, "no account lock before admission")
	require.NoError(t, closer.Commit(ctx))
	require.ErrorIs(t, <-done, mmadmission.ErrUnavailable)
	var receipts, accepted int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_response_receipts`).Scan(&receipts))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM match_proposals WHERE match_id=$1 AND response='accepted'`, intent.MatchID).Scan(&accepted))
	require.Zero(t, receipts)
	require.Zero(t, accepted)
}

func freshResponseProof(t *testing.T, pool *pgxpool.Pool, st *store.MatchStore, p store.CreateSourcePartyParams, params store.ResponseParams) (store.ResponseCandidate, *partysource.ValidatedResponseAttempt) {
	t.Helper()
	ctx := context.Background()
	c, err := st.CaptureResponseCandidate(ctx, params)
	require.NoError(t, err)
	v := &partysource.Revalidator{Snapshots: provisionReaderFunc(func(context.Context, partysource.Subject) (partysource.Snapshot, error) {
		require.Zero(t, pool.Stat().AcquiredConns())
		return partysource.Snapshot{ProtocolVersion: p.ProtocolVersion, Kind: p.SourceKind, RoomID: p.SourceRoomID, RosterVersion: p.SourceRosterVersion, MemberProfileIDs: p.MemberProfileIDs, MemberAuthorityVersion: p.MemberAuthorityVersion, MemberAuthorities: p.MemberAuthorities, SourceAuthorityHash: p.SourceAuthorityHash}, nil
	})}
	proof, err := v.ValidateResponseCandidate(ctx, c)
	require.NoError(t, err)
	require.NotNil(t, proof.Attempt)
	return c, proof.Attempt
}

func TestNativeResponseReceiptCommitsWithDecisionAndReplaysClosedWithoutDispatch(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	params := store.ResponseParams{MatchID: intent.MatchID, AccountID: p.MemberAuthorities[0].AccountID, ProfileID: p.MemberAuthorities[0].ProfileID, OperationID: uuid.New(), SessionEpoch: 99, RequestHash: [32]byte{1}, Accept: true}
	c, proof := freshResponseProof(t, pool, st, p, params)
	_, err = st.RecordValidatedResponse(ctx, c, nil)
	require.Error(t, err)
	_, err = pool.Exec(ctx, `CREATE FUNCTION response_receipt_failure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected receipt failure'; END $$; CREATE TRIGGER response_receipt_failure BEFORE INSERT ON matchmaking_match_response_receipts FOR EACH ROW EXECUTE FUNCTION response_receipt_failure()`)
	require.NoError(t, err)
	_, err = st.RecordValidatedResponse(ctx, c, proof)
	require.Error(t, err)
	var response string
	require.NoError(t, pool.QueryRow(ctx, `SELECT response FROM match_proposals WHERE match_id=$1 AND profile_id=$2`, intent.MatchID, params.ProfileID).Scan(&response))
	require.Equal(t, "pending", response, "receipt failure rolls back acceptance")
	_, err = pool.Exec(ctx, `DROP TRIGGER response_receipt_failure ON matchmaking_match_response_receipts;DROP FUNCTION response_receipt_failure()`)
	require.NoError(t, err)
	c, proof = freshResponseProof(t, pool, st, p, params)
	first, err := st.RecordValidatedResponse(ctx, c, proof)
	require.NoError(t, err)
	require.False(t, first.AllAccepted)
	require.False(t, first.Replayed)
	second := params
	second.AccountID = p.MemberAuthorities[1].AccountID
	second.ProfileID = p.MemberAuthorities[1].ProfileID
	second.OperationID = uuid.New()
	second.RequestHash = [32]byte{2}
	c, proof = freshResponseProof(t, pool, st, p, second)
	last, err := st.RecordValidatedResponse(ctx, c, proof)
	require.NoError(t, err)
	require.True(t, last.AllAccepted)
	require.False(t, last.Replayed)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	replayed, found, err := st.ReadResponseReceipt(ctx, second)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, replayed.Replayed)
	require.False(t, replayed.AllAccepted, "historical retry never requests provider redispatch")
	second.Accept = false
	second.RequestHash = [32]byte{9}
	_, _, err = st.ReadResponseReceipt(ctx, second)
	require.ErrorIs(t, err, store.ErrMatchOperationConflict)
	var count int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM matchmaking_match_response_receipts`).Scan(&count))
	require.Equal(t, 2, count)
}

func TestNativeDeclineReceiptRemainsTerminalWithClosedAdmission(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1`, intent.MatchID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE mm_native_admission_head SET state='CLOSED'`)
	require.NoError(t, err)
	params := store.ResponseParams{MatchID: intent.MatchID, AccountID: p.MemberAuthorities[0].AccountID, ProfileID: p.MemberAuthorities[0].ProfileID, OperationID: uuid.New(), SessionEpoch: 99, RequestHash: [32]byte{1}}
	c, err := st.CaptureResponseCandidate(ctx, params)
	require.NoError(t, err)
	result, err := st.RecordValidatedResponse(ctx, c, nil)
	require.NoError(t, err)
	require.Equal(t, store.MatchStatusAbandoned, result.Match.Status)
	require.False(t, result.AllAccepted)
	_, found, err := st.ReadResponseReceipt(ctx, params)
	require.NoError(t, err)
	require.True(t, found)
}

func TestNativeResponseDeadlineCommitsExpiryReceiptWithoutAcceptance(t *testing.T) {
	ctx := context.Background()
	pool, st, p, intent := admittedProvisionFixture(t)
	_, err := pool.Exec(ctx, `UPDATE match_proposals SET response='pending' WHERE match_id=$1;`, intent.MatchID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE matches SET created_at=clock_timestamp()-interval '1 hour' WHERE id=$1`, intent.MatchID)
	require.NoError(t, err)
	params := store.ResponseParams{MatchID: intent.MatchID, AccountID: p.MemberAuthorities[0].AccountID, ProfileID: p.MemberAuthorities[0].ProfileID, OperationID: uuid.New(), SessionEpoch: 99, RequestHash: [32]byte{1}, Accept: true}
	candidate, proof := freshResponseProof(t, pool, st, p, params)
	result, err := st.RecordValidatedResponse(ctx, candidate, proof)
	require.NoError(t, err)
	require.True(t, result.Expired)
	require.False(t, result.AllAccepted)
	require.Equal(t, store.MatchStatusAbandoned, result.Match.Status)
	var outcome string
	require.NoError(t, pool.QueryRow(ctx, `SELECT outcome FROM matchmaking_match_response_receipts WHERE operation_id=$1`, params.OperationID).Scan(&outcome))
	require.Equal(t, "EXPIRED", outcome)
	var accepted int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM match_proposals WHERE match_id=$1 AND response='accepted'`, intent.MatchID).Scan(&accepted))
	require.Zero(t, accepted)
	replay, found, err := st.ReadResponseReceipt(ctx, params)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, replay.Expired)
	require.False(t, replay.AllAccepted)
}
