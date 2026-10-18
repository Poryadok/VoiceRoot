package partysearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

func partyDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("real PostgreSQL unavailable: set BE011_TEST_PG_DSN")
	}
	ctx := context.Background()
	admin, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	database := "mm_party_" + uuid.New().String()[:8]
	_, err = admin.Exec(ctx, `CREATE DATABASE `+database)
	require.NoError(t, err)
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.ConnConfig.Database = database
	config.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, _ = admin.Exec(context.Background(), `DROP DATABASE `+database)
		admin.Close()
	})
	files, err := filepath.Glob("../../../migrations/matchmaking_db/*.up.sql")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 19)
	sort.Strings(files)
	for _, file := range files {
		body, err := os.ReadFile(file)
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, file)
	}
	return pool
}

func partyParams(t *testing.T, pool *pgxpool.Pool) store.CreateSourcePartyParams {
	t.Helper()
	profiles := []uuid.UUID{uuid.New(), uuid.New()}
	sort.Slice(profiles, func(i, j int) bool { return bytes.Compare(profiles[i][:], profiles[j][:]) < 0 })
	var game uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT id FROM games ORDER BY id LIMIT 1`).Scan(&game))
	room := uuid.New()
	return store.CreateSourcePartyParams{SearchOperationID: uuid.New(), SearchRequestHash: [32]byte{1}, InitiatorAccountID: uuid.New(), InitiatorProfileID: profiles[0], InitiatorSessionEpoch: 7, ProtocolVersion: 1, SourceKind: "VOICE_ROSTER", SourceRoomID: &room, SourceRosterVersion: 2, MemberProfileIDs: profiles, GameID: game, Mode: "5v5 Ranked", Criteria: `{"region":"eu"}`, TimeoutAt: time.Now().UTC().Add(30 * time.Minute)}
}

func TestSourcePartyCreatesEverySessionAndDurableIntentAtomically(t *testing.T) {
	pool := partyDB(t)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	result, err := parties.CreateSourceParty(context.Background(), p)
	require.NoError(t, err)
	require.Len(t, result.Sessions, 2)
	require.False(t, result.Replay)
	require.Equal(t, p.MemberProfileIDs, result.Party.MemberProfileIDs)
	require.Equal(t, p.InitiatorAccountID, result.Party.InitiatorAccountID)
	require.Equal(t, p.InitiatorProfileID, result.Party.LeaderProfileID)
	var manifest []byte
	for _, id := range p.MemberProfileIDs {
		manifest = append(manifest, id[:]...)
	}
	require.Equal(t, sha256.Sum256(manifest), result.Party.MemberManifestHash)
	for _, session := range result.Sessions {
		require.NotNil(t, session.PartyID)
		require.Equal(t, result.Party.ID, *session.PartyID)
		require.Equal(t, store.SessionStatusSearching, session.Status)
	}
	var action string
	var revision int64
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT action,revision FROM matchmaking_party_search_queue_intents WHERE party_id=$1`, result.Party.ID).Scan(&action, &revision))
	require.Equal(t, "ENQUEUE", action)
	require.Equal(t, int64(1), revision)
}

func TestSourcePartyMemberConflictRollsBackEarlierMemberPartyAndIntent(t *testing.T) {
	pool := partyDB(t)
	p := partyParams(t, pool)
	_, err := (&store.SessionStore{Pool: pool}).Create(context.Background(), store.CreateSessionParams{ProfileID: p.MemberProfileIDs[1], GameID: p.GameID, Mode: p.Mode, Criteria: p.Criteria, TimeoutAt: p.TimeoutAt})
	require.NoError(t, err)
	_, err = (&store.PartyStore{Pool: pool}).CreateSourceParty(context.Background(), p)
	require.ErrorIs(t, err, store.ErrActiveSearchExists)
	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM parties WHERE search_operation_id=$1`, p.SearchOperationID).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM search_sessions WHERE profile_id=$1`, p.MemberProfileIDs[0]).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT count(*) FROM matchmaking_party_search_queue_intents`).Scan(&count))
	require.Zero(t, count)
}

func TestSourcePartyConcurrentOverlapHasOneCompleteWinner(t *testing.T) {
	pool := partyDB(t)
	a := partyParams(t, pool)
	b := partyParams(t, pool)
	b.MemberProfileIDs = []uuid.UUID{a.MemberProfileIDs[1], uuid.New()}
	sort.Slice(b.MemberProfileIDs, func(i, j int) bool { return bytes.Compare(b.MemberProfileIDs[i][:], b.MemberProfileIDs[j][:]) < 0 })
	b.InitiatorProfileID = b.MemberProfileIDs[0]
	start := make(chan struct{})
	errors := make(chan error, 2)
	var wg sync.WaitGroup
	for _, p := range []store.CreateSourcePartyParams{a, b} {
		wg.Add(1)
		go func(p store.CreateSourcePartyParams) {
			defer wg.Done()
			<-start
			_, err := (&store.PartyStore{Pool: pool}).CreateSourceParty(context.Background(), p)
			errors <- err
		}(p)
	}
	close(start)
	wg.Wait()
	close(errors)
	wins := 0
	for err := range errors {
		if err == nil {
			wins++
		} else {
			require.ErrorIs(t, err, store.ErrActiveSearchExists)
		}
	}
	require.Equal(t, 1, wins)
	var parties, sessions, intents int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM parties),(SELECT count(*) FROM search_sessions),(SELECT count(*) FROM matchmaking_party_search_queue_intents)`).Scan(&parties, &sessions, &intents))
	require.Equal(t, 1, parties)
	require.Equal(t, 2, sessions)
	require.Equal(t, 1, intents)
}

func TestSourcePartyExactReplayAndConflictingOperation(t *testing.T) {
	pool := partyDB(t)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	first, err := parties.CreateSourceParty(context.Background(), p)
	require.NoError(t, err)
	replayed, err := parties.CreateSourceParty(context.Background(), p)
	require.NoError(t, err)
	require.True(t, replayed.Replay)
	require.Equal(t, first.Party.ID, replayed.Party.ID)
	require.Equal(t, first.Sessions, replayed.Sessions)
	p.SearchRequestHash[0]++
	_, err = parties.CreateSourceParty(context.Background(), p)
	require.ErrorIs(t, err, store.ErrSearchOperationConflict)
}

func TestSourcePartyCapturedRosterCannotBeRewrittenByLFPAndClosedPartyCannotReplay(t *testing.T) {
	pool := partyDB(t)
	p := partyParams(t, pool)
	parties := &store.PartyStore{Pool: pool}
	created, err := parties.CreateSourceParty(context.Background(), p)
	require.NoError(t, err)
	_, err = pool.Exec(context.Background(), `UPDATE parties SET member_profile_ids='[]'::jsonb WHERE id=$1`, created.Party.ID)
	require.Error(t, err)
	_, err = pool.Exec(context.Background(), `UPDATE parties SET disbanded_at=clock_timestamp() WHERE id=$1`, created.Party.ID)
	require.NoError(t, err)
	_, err = parties.CreateSourceParty(context.Background(), p)
	require.ErrorIs(t, err, store.ErrSourcePartyChanged)
}
