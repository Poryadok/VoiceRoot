//go:build spacebanintegration

package store

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Administrative phase: this explicit source fixture must already be enrolled.
// A no-op PUBLIC source mutation still observes sources and invalidates frame
// completion, while unchanged private sticky flags must retain their tuple.
func TestNativeObserverNoopTupleAndFrame(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	st := &SpaceStore{Pool: pool}
	space, profile := nativeBanSpace(19), nativeBanProfile(3)
	err = st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{profile})
		require.NoError(t, err)
		require.NotNil(t, capture)
		require.NotNil(t, capture.dbMutation)
		require.NoError(t, scoped.finishEffectiveMemberMutations(ctx, capture))
		var before, after string
		var sequence, completed, afterSequence, afterCompleted int64
		var beforeAt, afterAt, completedAt *time.Time
		var digest []byte
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ctid::text FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile).Scan(&before))
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT sequence,completed_sequence,last_source_at FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=$1`, space).Scan(&sequence, &completed, &beforeAt))
		require.Equal(t, sequence, completed)
		_, err = scoped.tx.Exec(ctx, `UPDATE public.space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space, profile)
		require.NoError(t, err)
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ctid::text FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile).Scan(&after))
		require.Equal(t, before, after, "a genuine unchanged callback must not create a redundant private sticky-cut tuple")
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT sequence,completed_sequence,last_source_at,completed_at,completed_digest FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=$1`, space).Scan(&afterSequence, &afterCompleted, &afterAt, &completedAt, &digest))
		require.Greater(t, afterSequence, sequence, "no-op source callback remains a real frame mutation")
		require.EqualValues(t, -1, afterCompleted)
		require.Nil(t, completedAt)
		require.Nil(t, digest)
		require.NotNil(t, afterAt)
		if beforeAt != nil {
			require.True(t, afterAt.After(*beforeAt), "preserve the unconditional fresh frame timestamp")
		}
		require.ErrorIs(t, scoped.finishEffectiveMemberMutations(ctx, capture), ErrEffectiveEpisodeUnavailable, "a consumed capture is never a second completion authority")
		return nil // Exercise the real deferred commit guard after late dirtying.
	})
	var pgErr *pgconn.PgError
	require.ErrorAs(t, err, &pgErr)
	require.Equal(t, "55000", pgErr.Code)
	require.Contains(t, pgErr.Message, "complet", "late no-op source mutation invalidates the earlier real completion")
}

// The boolean/physical algebra fixture executes the actual old and new embedded
// UPDATE statements on a real own-xid capture row. Owner-only flag seeding is
// test data, never an identity receipt or a native completion/publication.
func TestNativeObserverCutPredicateAlgebra(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	statement := func(definition string) string {
		t.Helper()
		_, tail, ok := strings.Cut(definition, "UPDATE native_membership_guard.profiles SET")
		require.True(t, ok)
		tail, _, ok = strings.Cut(tail, ";")
		require.True(t, ok)
		return strings.NewReplacer("r.old_sources", "($3::jsonb)", "src->>", "($4::jsonb)->>", "xid=r.xid", "xid=txid_current()", "space_id=s", "space_id=$1", "profile_id=r.profile_id", "profile_id=$2").Replace("UPDATE native_membership_guard.profiles SET" + tail)
	}
	oldSQL, newSQL := statement(nativeObserverPrior50Definition), statement(nativeObserver50Definition)
	space, profile := nativeBanSpace(19), nativeBanProfile(3)
	rollback := errors.New("cut algebra fixture rollback")
	st := &SpaceStore{Pool: pool}
	err = st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{profile})
		require.NoError(t, err)
		require.NotNil(t, capture)
		for flags := 0; flags < 4; flags++ {
			for losses := 0; losses < 4; losses++ {
				old := map[string]any{"ordinary": "O", "community": "C"}
				current := map[string]any{"ordinary": "O", "community": "C"}
				if losses&1 != 0 {
					current["ordinary"] = nil
				}
				if losses&2 != 0 {
					current["community"] = "different witness"
				}
				oldJSON, err := json.Marshal(old)
				require.NoError(t, err)
				currentJSON, err := json.Marshal(current)
				require.NoError(t, err)
				seed := func() string {
					t.Helper()
					_, err := scoped.tx.Exec(ctx, `UPDATE native_membership_guard.profiles SET ordinary_cut=$3,community_cut=$4 WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile, flags&1 != 0, flags&2 != 0)
					require.NoError(t, err)
					var tuple string
					require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ctid::text FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile).Scan(&tuple))
					return tuple
				}
				seed()
				_, err = scoped.tx.Exec(ctx, oldSQL, space, profile, oldJSON, currentJSON)
				require.NoError(t, err)
				var expectedO, expectedC, actualO, actualC bool
				require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ordinary_cut,community_cut FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile).Scan(&expectedO, &expectedC))
				before := seed()
				tag, err := scoped.tx.Exec(ctx, newSQL, space, profile, oldJSON, currentJSON)
				require.NoError(t, err)
				var after string
				require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ordinary_cut,community_cut,ctid::text FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile).Scan(&actualO, &actualC, &after))
				require.Equal(t, expectedO, actualO, "flags=%d losses=%d ordinary", flags, losses)
				require.Equal(t, expectedC, actualC, "flags=%d losses=%d community", flags, losses)
				changed := expectedO != (flags&1 != 0) || expectedC != (flags&2 != 0)
				require.Equal(t, changed, tag.RowsAffected() == 1)
				require.Equal(t, changed, before != after, "only actual false→true state transitions create tuples")
			}
		}
		// NULL old witnesses never cut, even if a new witness appears.
		_, err = scoped.tx.Exec(ctx, `UPDATE native_membership_guard.profiles SET ordinary_cut=false,community_cut=false WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, profile)
		require.NoError(t, err)
		for _, current := range []string{`{"ordinary":null,"community":null}`, `{"ordinary":"new","community":"new"}`} {
			tag, err := scoped.tx.Exec(ctx, newSQL, space, profile, `{"ordinary":null,"community":null}`, current)
			require.NoError(t, err)
			require.Zero(t, tag.RowsAffected())
		}
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// A real owner-lease gap is sampled by an unrelated ordinary callback on BOTH
// supported engines. Already-cut rows still visit sources, but do not churn
// tuples on repeated observations or after the same witness is restored.
func TestNativeObserverNoopLeaseGapTuple(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	st := &SpaceStore{Pool: pool}
	rollback := errors.New("real lease-gap tuple rollback")
	for _, gap := range []bool{false, true} {
		space, ordinary, community := nativeBanSpace(19), nativeBanProfile(3), nativeBanProfile(5)
		var finalErr error
		err := st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
			capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{ordinary})
			require.NoError(t, err)
			require.NotNil(t, capture)
			var expires time.Time
			require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT clock_timestamp()+interval '500 milliseconds'`).Scan(&expires))
			_, err = scoped.tx.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=$2 WHERE space_id=$1`, space, expires)
			require.NoError(t, err)
			if gap {
				waitUntil := time.Now().Add(time.Second)
				for {
					var expired bool
					require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT clock_timestamp()>$1`, expires).Scan(&expired))
					if expired {
						break
					}
					require.True(t, time.Now().Before(waitUntil), "real lease clock must advance within the bounded test wait")
					time.Sleep(10 * time.Millisecond)
				}
			}
			var before, after, repeated string
			var cut bool
			require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ctid::text FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, community).Scan(&before))
			_, err = scoped.tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space, ordinary)
			require.NoError(t, err)
			require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ctid::text,community_cut FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, community).Scan(&after, &cut))
			require.Equal(t, gap, cut)
			require.Equal(t, gap, before != after)
			_, err = scoped.tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space, ordinary)
			require.NoError(t, err)
			require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT ctid::text FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, community).Scan(&repeated))
			require.Equal(t, after, repeated, "repeated existing cuts remain sticky without another tuple")
			_, err = scoped.tx.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=clock_timestamp()+interval '10 seconds' WHERE space_id=$1`, space)
			require.NoError(t, err)
			finalErr = scoped.finishEffectiveMemberMutations(ctx, capture)
			return rollback
		})
		require.ErrorIs(t, err, rollback)
		if gap {
			require.ErrorIs(t, finalErr, ErrEffectiveEpisodeUnavailable)
		} else {
			require.NoError(t, finalErr)
		}
	}
}

func TestNativeObserverPrivateClosureAndLock(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("SPACE_BAN_TEST_DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	st := &SpaceStore{Pool: pool}
	space, profile := nativeBanSpace(19), nativeBanProfile(3)
	rollback := errors.New("private observer closure rollback")
	err = st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
		capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{profile})
		require.NoError(t, err)
		require.NotNil(t, capture)
		_, err = scoped.tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space, profile)
		require.NoError(t, err)
		var locked bool
		require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND relation='native_membership_guard.profiles'::regclass AND mode='RowExclusiveLock' AND granted)`).Scan(&locked))
		require.True(t, locked, "capture and retained zero-row UPDATE preserve the relation lock")
		other, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer other.Rollback(ctx)
		_, err = other.Exec(ctx, `SET LOCAL lock_timeout='50ms'`)
		require.NoError(t, err)
		_, err = other.Exec(ctx, `ALTER TABLE native_membership_guard.profiles ADD COLUMN unexpected_lock_probe boolean`)
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr)
		require.Equal(t, "55P03", pgErr.Code, "another backend cannot use a no-op window for private relation DDL")
		require.NoError(t, other.Rollback(ctx))
		require.NoError(t, scoped.finishEffectiveMemberMutations(ctx, capture))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	for _, ddl := range []string{
		`GRANT SELECT ON native_membership_guard.profiles TO PUBLIC`,
		`ALTER TABLE native_membership_guard.profiles ENABLE ROW LEVEL SECURITY`,
		`CREATE TRIGGER unexpected_profiles_audit AFTER UPDATE ON native_membership_guard.profiles FOR EACH ROW EXECUTE FUNCTION native_membership_guard.commit_check()`,
	} {
		err := st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
			require.NoError(t, checkMemberAuthorityCatalog(ctx, scoped.tx))
			_, err := scoped.tx.Exec(ctx, `SELECT native_membership_guard.scope($1)`, space)
			require.NoError(t, err, "the installer has the genuine backend fence and exact current catalog")
			_, err = scoped.tx.Exec(ctx, ddl)
			require.NoError(t, err, "install the actual metadata counterexample")
			err = checkMemberAuthorityCatalog(ctx, scoped.tx)
			if err == nil {
				_, err = scoped.tx.Exec(ctx, `SELECT native_membership_guard.scope($1)`, space)
			}
			require.Error(t, err, "unexpected ACL/RLS/trigger side effects remain fail-closed")
			return rollback
		})
		require.ErrorIs(t, err, rollback)
	}
	serving := projectionServingLogin(t, ctx, pool)
	_, err = serving.Exec(ctx, `UPDATE native_membership_guard.profiles SET ordinary_cut=true`)
	var denied *pgconn.PgError
	require.ErrorAs(t, err, &denied)
	require.Equal(t, "42501", denied.Code, "serving has no direct private capture mutation privilege")
}
