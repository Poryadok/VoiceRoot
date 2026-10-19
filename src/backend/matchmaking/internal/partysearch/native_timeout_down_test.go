package partysearch

import (
	"context"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/store"
)

func TestNativeTimeoutDownRefusesPinnedRRAndRoundTripsEmptyRC(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	down, err := os.ReadFile("../../../migrations/matchmaking_db/000027_native_party_timeout.down.sql")
	require.NoError(t, err)
	up, err := os.ReadFile("../../../migrations/matchmaking_db/000027_native_party_timeout.up.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err, "actual never-begun empty RC rollback")
	_, err = pool.Exec(ctx, string(up))
	require.NoError(t, err)
	pinned, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer pinned.Rollback(ctx)
	var absent int
	require.NoError(t, pinned.QueryRow(ctx, `SELECT count(*) FROM mm_native_admission_head`).Scan(&absent))
	require.Zero(t, absent)
	_, _, _ = historyReceiptFixture(t, pool)
	_, err = pinned.Exec(ctx, string(down))
	require.ErrorContains(t, err, "requires fresh read committed checks")
	require.NoError(t, pinned.Rollback(ctx))
	var exists bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('matchmaking_native_timeout_outbox') IS NOT NULL`).Scan(&exists))
	require.True(t, exists)
}

func TestNativeTimeoutDownRetainsUnknownFirstWriterCommittedDuringWait(t *testing.T) {
	ctx := context.Background()
	pool := partyDB(t)
	p := partyParams(t, pool)
	// Actual pre-cut typed history with no native admission/origin. Unknown
	// timeout bytes must still be retained; this is not an expiry authority proof.
	party, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(ctx, p)
	require.NoError(t, err)
	down, err := os.ReadFile("../../../migrations/matchmaking_db/000027_native_party_timeout.down.sql")
	require.NoError(t, err)
	writer, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer writer.Rollback(ctx)
	_, err = writer.Exec(ctx, `INSERT INTO matchmaking_native_timeout_outbox(event_id,party_id,search_session_id,profile_id,terminal_generation,occurred_at,payload) VALUES($1,$2,$3,$4,1,clock_timestamp(),$5)`, uuid.New(), party.Party.ID, party.Sessions[0].ID, party.Sessions[0].ProfileID, []byte{1})
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() {
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_, err := pool.Exec(bounded, string(down))
		done <- err
	}()
	require.Eventually(t, func() bool {
		select {
		case err := <-done:
			t.Fatalf("DOWN bypassed uncommitted evidence: %v", err)
		default:
		}
		var waiting bool
		return pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE '%LOCK TABLE public.mm_native_admission_head%matchmaking_native_timeout_outbox%')`).Scan(&waiting) == nil && waiting
	}, 3*time.Second, 10*time.Millisecond)
	require.NoError(t, writer.Commit(ctx))
	require.ErrorContains(t, <-done, "timeout and admission evidence must be retained")
	var payload []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT payload FROM matchmaking_native_timeout_outbox`).Scan(&payload))
	require.Equal(t, []byte{1}, payload)
}
