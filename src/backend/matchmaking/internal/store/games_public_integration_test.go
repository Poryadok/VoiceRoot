package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func TestPublicGamesSQLRankPrivacyAndPaging(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL")
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	_, err := pool.Exec(ctx, "TRUNCATE games CASCADE")
	require.NoError(t, err)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	var cutoff time.Time
	require.NoError(t, tx.QueryRow(ctx, "SELECT CURRENT_TIMESTAMP").Scan(&cutoff))
	future := cutoff.Add(time.Hour)
	a, b, c, d, pending := uuid.New(), uuid.New(), uuid.MustParse("00000000-0000-0000-0000-000000000001"), uuid.MustParse("00000000-0000-0000-0000-000000000002"), uuid.New()
	created := cutoff.Add(-24 * time.Hour)
	for i, id := range []uuid.UUID{a, b, c, d, pending} {
		status := StatusActive
		if id == pending {
			status = StatusPendingModeration
		}
		at := created.Add(time.Duration(min(i, 2)) * time.Hour)
		_, err := tx.Exec(ctx, `INSERT INTO games(id,name,config,status,created_at) VALUES($1,$2,'{}',$3,$4)`, id, id.String(), status, at)
		require.NoError(t, err)
	}
	add := func(game uuid.UUID, status string, deadline any, space any) uuid.UUID {
		id := uuid.New()
		_, err := tx.Exec(ctx, `INSERT INTO search_sessions(id,profile_id,game_id,mode,criteria,status,timeout_at,space_id)
			VALUES($1,$2,$3,$4,$5::jsonb,$6,$7,$8)`, id, uuid.New(), game,
			"mode-"+id.String(), `{"region":"eu"}`, status, deadline, space)
		require.NoError(t, err)
		return id
	}
	// Different mode/region sessions (including a still-live nudged session)
	// aggregate by game. Each durable participant session contributes once.
	a1 := add(a, SessionStatusSearching, future, nil)
	_, err = tx.Exec(ctx, `UPDATE search_sessions SET nudged_at=$2, criteria='{"region":"na"}' WHERE id=$1`, a1, cutoff.Add(-15*time.Minute))
	require.NoError(t, err)
	a2 := add(a, SessionStatusSearching, future, nil)
	party := uuid.New()
	_, err = tx.Exec(ctx, `INSERT INTO parties(id,leader_profile_id,game_id,mode,criteria) VALUES($1,$2,$3,'party','{}')`, party, uuid.New(), a)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, "UPDATE search_sessions SET party_id=$1 WHERE id=ANY($2)", party, []uuid.UUID{a1, a2})
	require.NoError(t, err)
	add(b, SessionStatusSearching, future, nil)
	for _, status := range []string{SessionStatusPendingAccept, SessionStatusMatched, SessionStatusCancelled, SessionStatusTimeout} {
		add(c, status, future, nil)
	}
	add(c, SessionStatusSearching, cutoff, nil) // equality is expired at the fixed DB cutoff
	add(c, SessionStatusSearching, cutoff.Add(-time.Second), nil)
	add(c, SessionStatusSearching, nil, nil)
	for i := 0; i < 8; i++ {
		add(c, SessionStatusSearching, future, uuid.New())
		add(pending, SessionStatusSearching, future, nil)
	}
	all, err := listPublicGames(ctx, tx, ListGamesParams{})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{a, b, d, c}, publicGameIDs(all.Games))
	first, err := listPublicGames(ctx, tx, ListGamesParams{PageSize: 1})
	require.NoError(t, err)
	add(c, SessionStatusSearching, future, uuid.New())
	spaceChanged, err := listPublicGames(ctx, tx, ListGamesParams{PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, first.NextCursor, spaceChanged.NextCursor, "Space activity cannot affect the public fingerprint")
	add(a, SessionStatusSearching, future, nil) // score changes, order does not
	second, err := listPublicGames(ctx, tx, ListGamesParams{Cursor: first.NextCursor, PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, b, second.Games[0].ID)
	var seen []uuid.UUID
	cursor := ""
	for {
		page, err := listPublicGames(ctx, tx, ListGamesParams{Cursor: cursor, PageSize: 1})
		require.NoError(t, err)
		seen = append(seen, publicGameIDs(page.Games)...)
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
		require.Less(t, len(seen), 5, "traversal must terminate")
	}
	require.Equal(t, []uuid.UUID{a, b, d, c}, seen)
	for i := 0; i < 4; i++ {
		add(b, SessionStatusSearching, future, nil)
	}
	stale, err := listPublicGames(ctx, tx, ListGamesParams{Cursor: first.NextCursor})
	require.ErrorIs(t, err, ErrGamesCursorStale)
	require.Empty(t, stale.Games)
	require.NoError(t, tx.Commit(ctx))

	s := &GameStore{Pool: pool}
	staff, err := s.List(ctx, ListGamesParams{Status: StatusActive})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{d, c, b, a}, publicGameIDs(staff.Games))
	legacy, err := s.ListPublic(ctx, ListGamesParams{Cursor: encodeCursor(created.Add(2*time.Hour), d), PageSize: 2})
	require.NoError(t, err)
	require.Equal(t, []uuid.UUID{c, b}, publicGameIDs(legacy.Games))
	require.Contains(t, legacy.NextCursor, "|")
	_, err = s.List(ctx, ListGamesParams{Cursor: first.NextCursor})
	require.Error(t, err, "staff path must not accept the public token")
	active, err := s.ListPublic(ctx, ListGamesParams{PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, b, active.Games[0].ID)
	_, err = pool.Exec(ctx, "UPDATE games SET status='archived' WHERE id=$1", c)
	require.NoError(t, err)
	_, err = s.ListPublic(ctx, ListGamesParams{Cursor: active.NextCursor})
	require.ErrorIs(t, err, ErrGamesCursorStale)
	_, err = pool.Exec(ctx, "UPDATE search_sessions SET status='cancelled'")
	require.NoError(t, err)
	zero, err := s.ListPublic(ctx, ListGamesParams{})
	require.NoError(t, err)
	chronological, err := s.List(ctx, ListGamesParams{Status: StatusActive})
	require.NoError(t, err)
	require.Equal(t, publicGameIDs(chronological.Games), publicGameIDs(zero.Games))

	// Record the actual plan when this SQL test executes; no index or benchmark
	// success is inferred from merely compiling this test.
	var plan string
	require.NoError(t, pool.QueryRow(ctx, "EXPLAIN (ANALYZE, FORMAT JSON) "+publicGamesQuery).Scan(&plan))
	require.Contains(t, plan, "Execution Time")
	t.Logf("public catalog SQL plan: %s", plan)
}

type publicGamesAfterQuery struct {
	pool  *pgxpool.Pool
	after func()
}

func (q publicGamesAfterQuery) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	rows, err := q.pool.Query(ctx, query, args...)
	if err == nil {
		q.after()
	}
	return rows, err
}

func TestPublicGamesSQLConcurrentCommitUsesOneOrderSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("requires real PostgreSQL")
	}
	ctx := context.Background()
	pool := StartMatchmakingDBForStoreTest(t, ctx)
	ApplyMatchmakingMigrationsForStoreTest(t, ctx, pool)
	_, err := pool.Exec(ctx, "TRUNCATE games CASCADE")
	require.NoError(t, err)
	s := &GameStore{Pool: pool}
	a, err := s.Create(ctx, "first", validTestConfig(), uuid.New())
	require.NoError(t, err)
	b, err := s.Create(ctx, "second", validTestConfig(), uuid.New())
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "UPDATE games SET created_at='2020-01-01'::timestamptz WHERE id=$1", a.ID)
	require.NoError(t, err)
	// B sorts first at query time; A gains a searching session after the SELECT
	// snapshot was opened but before the caller reads/hashes the rows.
	q := publicGamesAfterQuery{pool: pool, after: func() {
		_, err := pool.Exec(ctx, `INSERT INTO search_sessions(id,profile_id,game_id,mode,criteria,timeout_at)
			VALUES($1,$2,$3,'test','{}',CURRENT_TIMESTAMP + INTERVAL '1 hour')`, uuid.New(), uuid.New(), a.ID)
		require.NoError(t, err)
	}}
	page, err := listPublicGames(ctx, q, ListGamesParams{PageSize: 1})
	require.NoError(t, err)
	require.Equal(t, b.ID, page.Games[0].ID)
	_, err = s.ListPublic(ctx, ListGamesParams{Cursor: page.NextCursor})
	require.ErrorIs(t, err, ErrGamesCursorStale)
	fresh, err := s.ListPublic(ctx, ListGamesParams{PageSize: 1})
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "INSERT INTO games(name,config,status) VALUES('new','{}','active')")
	require.NoError(t, err)
	_, err = s.ListPublic(ctx, ListGamesParams{Cursor: fresh.NextCursor})
	require.ErrorIs(t, err, ErrGamesCursorStale)
}

func publicGameIDs(games []Game) []uuid.UUID {
	ids := make([]uuid.UUID, len(games))
	for i, game := range games {
		ids[i] = game.ID
	}
	return ids
}

func TestPublicGamesQueryFailureDoesNotInventPopularity(t *testing.T) {
	// A missing local dependency is an error, never a successful empty or
	// chronological fallback. Strict parsing still precedes database access.
	s := &GameStore{}
	_, err := s.ListPublic(context.Background(), ListGamesParams{})
	require.Error(t, err)
	_, err = s.ListPublic(context.Background(), ListGamesParams{Cursor: strings.Repeat("x", 129)})
	require.ErrorIs(t, err, ErrGamesCursorInvalid)
	_, err = s.ListPublic(context.Background(), ListGamesParams{Cursor: encodeCursor(time.Now(), uuid.New())})
	require.Error(t, err)
}
