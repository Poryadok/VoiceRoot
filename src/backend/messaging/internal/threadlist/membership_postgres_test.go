package threadlist

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func membershipDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("THREADLIST_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("requires owned THREADLIST_TEST_DATABASE_URL")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	schema := "threadmembership_" + uuid.NewString()
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize())
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE")
		pool.Close()
		require.NoError(t, err)
	})
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	for _, migration := range []string{"000032_thread_list_nodes.up.sql", "000039_thread_list_membership.up.sql", "000040_thread_membership_baselines.up.sql"} {
		raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "../../../migrations/messaging_db", migration))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(raw))
		require.NoError(t, err)
	}
	return pool
}

func TestPostgresMembershipReorderedPrefixAndRetry(t *testing.T) {
	pool := membershipDB(t)
	inbox := MembershipInbox{Pool: pool}
	ctx := context.Background()
	join := membershipFixture()
	require.NoError(t, inbox.Track(ctx, join.ChatID, join.Incarnation))
	remove := join
	remove.EventID, remove.Offset, remove.Revision, remove.Kind = uuid.New(), 2, 2, MemberRemoved
	require.NoError(t, inbox.Apply(ctx, remove))
	_, err := inbox.Pair(ctx, join.ChatID, join.Incarnation, join.ProfileID, 2)
	require.ErrorIs(t, err, ErrMembershipUnavailable, "a received high offset is not a consumed prefix")
	require.NoError(t, inbox.Apply(ctx, join))
	pair, err := inbox.Pair(ctx, join.ChatID, join.Incarnation, join.ProfileID, 2)
	require.NoError(t, err)
	require.Equal(t, MemberRevoked, pair.Status)
	require.EqualValues(t, 2, pair.Revision)
	rejoin := join
	rejoin.EventID, rejoin.Offset, rejoin.Revision, rejoin.Lineage = uuid.New(), 3, 3, uuid.New()
	require.NoError(t, inbox.Apply(ctx, rejoin))
	require.NoError(t, inbox.Apply(ctx, remove), "late exact delivery remains idempotent")
	pair, err = inbox.Pair(ctx, join.ChatID, join.Incarnation, join.ProfileID, 3)
	require.NoError(t, err)
	require.Equal(t, MemberBuilding, pair.Status)
	require.Equal(t, rejoin.Lineage, pair.Lineage)
	// Unknown source identity cannot reset a tracked committed prefix.
	require.ErrorIs(t, inbox.Track(ctx, join.ChatID, uuid.New()), ErrMembershipUnavailable)
	_, err = inbox.Pair(ctx, join.ChatID, join.Incarnation, join.ProfileID, 4)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
}

func TestPostgresMembershipConflictSurvivesRestart(t *testing.T) {
	for _, collision := range []string{"event", "offset", "pair_revision", "retired_lineage", "source_vector", "malformed_repeat"} {
		t.Run(collision, func(t *testing.T) {
			pool := membershipDB(t)
			ctx := context.Background()
			inbox := MembershipInbox{Pool: pool}
			e := membershipFixture()
			require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
			require.NoError(t, inbox.Apply(ctx, e))
			bad := e
			switch collision {
			case "source_vector":
				bad.Sources = append([]MembershipSource{}, e.Sources...)
				bad.Sources[0].RevocationRevision++
			case "malformed_repeat":
				bad.Kind = "unknown"
			case "event":
				bad.Kind = MemberRemoved
			case "offset":
				bad.EventID, bad.ProfileID = uuid.New(), uuid.New()
			case "pair_revision":
				bad.EventID, bad.Offset, bad.Kind = uuid.New(), 2, MemberRemoved
			case "retired_lineage":
				bad.EventID, bad.Offset, bad.Revision, bad.Kind = uuid.New(), 2, 2, MemberRemoved
				require.NoError(t, inbox.Apply(ctx, bad))
				bad.EventID, bad.Offset, bad.Revision, bad.Kind, bad.Lineage = uuid.New(), 3, 3, MemberJoined, uuid.New()
				require.NoError(t, inbox.Apply(ctx, bad))
				bad.EventID, bad.Offset, bad.Revision, bad.Kind = uuid.New(), 4, 4, MemberLeft
				require.NoError(t, inbox.Apply(ctx, bad))
				bad.EventID, bad.Offset, bad.Revision, bad.Kind, bad.Lineage = uuid.New(), 5, 5, MemberJoined, e.Lineage
			}
			require.ErrorIs(t, inbox.Apply(ctx, bad), ErrMembershipConflict)
			restarted := MembershipInbox{Pool: pool}
			_, err := restarted.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 1)
			require.ErrorIs(t, err, ErrMembershipUnavailable)
			require.ErrorIs(t, restarted.Apply(ctx, e), ErrMembershipUnavailable, "exact retry cannot clear quarantine")
		})
	}
}

func TestPostgresMembershipConflictingConcurrentDelivery(t *testing.T) {
	pool := membershipDB(t)
	ctx := context.Background()
	inbox := MembershipInbox{Pool: pool}
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	other := e
	other.EventID, other.ProfileID = uuid.New(), uuid.New()
	other.Sources = append([]MembershipSource{}, e.Sources...)
	other.Sources[1].Scope = other.ProfileID
	start, result := make(chan struct{}), make(chan error, 2)
	for _, record := range []MembershipRecord{e, other} {
		go func(record MembershipRecord) { <-start; result <- inbox.Apply(ctx, record) }(record)
	}
	close(start)
	a, b := <-result, <-result
	require.True(t, (a == nil && errors.Is(b, ErrMembershipConflict)) || (b == nil && errors.Is(a, ErrMembershipConflict)))
	_, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 1)
	require.ErrorIs(t, err, ErrMembershipUnavailable)
}

func TestPostgresMembershipEquivalentVectorOrderAndInvalidNovelEvent(t *testing.T) {
	pool := membershipDB(t)
	ctx := context.Background()
	inbox := MembershipInbox{Pool: pool}
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	bad := e
	bad.Kind = "unknown"
	require.ErrorIs(t, inbox.Apply(ctx, bad), ErrInvalidMembership)
	require.NoError(t, inbox.Apply(ctx, e), "invalid novel record must not poison a known scope")
	reordered := e
	reordered.Sources = []MembershipSource{e.Sources[1], e.Sources[0]}
	require.NoError(t, inbox.Apply(ctx, reordered), "source coordinates are a set, not transport order")
}

func TestPostgresMembershipRollbackAndConcurrentRetry(t *testing.T) {
	pool := membershipDB(t)
	ctx := context.Background()
	inbox := MembershipInbox{Pool: pool}
	e := membershipFixture()
	require.NoError(t, inbox.Track(ctx, e.ChatID, e.Incarnation))
	_, err := pool.Exec(ctx, `CREATE FUNCTION reject_viewer() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected viewer failure'; END $$;
CREATE TRIGGER reject_viewer BEFORE INSERT ON thread_list_membership_pairs FOR EACH ROW EXECUTE FUNCTION reject_viewer()`)
	require.NoError(t, err)
	require.ErrorContains(t, inbox.Apply(ctx, e), "injected viewer failure")
	var count, prefix int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM thread_list_membership_inbox`).Scan(&count))
	require.Zero(t, count, "inbox insert rolls back with viewer and prefix")
	require.NoError(t, pool.QueryRow(ctx, `SELECT applied_q FROM thread_list_membership_heads WHERE chat_id=$1`, e.ChatID).Scan(&prefix))
	require.Zero(t, prefix)
	_, err = pool.Exec(ctx, `DROP TRIGGER reject_viewer ON thread_list_membership_pairs`)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errors := make(chan error, 8)
	for range 8 {
		wg.Go(func() { errors <- inbox.Apply(ctx, e) })
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM thread_list_membership_inbox`).Scan(&count))
	require.EqualValues(t, 1, count)
	pair, err := inbox.Pair(ctx, e.ChatID, e.Incarnation, e.ProfileID, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, pair.Revision)
}
