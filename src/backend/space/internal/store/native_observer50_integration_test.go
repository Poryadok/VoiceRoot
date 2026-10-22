//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Run before the explicit source50 transition, while the retained seal is the
// genuine original45 value and no serving grants or enrollment have occurred.
// Every metadata/body counterexample rolls back inside its own transaction.
func TestNativeObserver50DefinitionAndPreimage(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer tx.Rollback(ctx)
	require.NoError(t, checkMemberAuthorityCatalog(ctx, tx))
	oid, original, err := requireNativeObserver50Definition(ctx, tx)
	require.NoError(t, err)
	catalogOID, originalCatalog, err := requireNativeCatalog50Definition(ctx, tx)
	require.NoError(t, err)
	var retained, reconstructed, current []byte
	require.NoError(t, tx.QueryRow(ctx, `SELECT catalog_sha256,native_membership_guard.catalog() FROM native_membership_guard.epoch WHERE singleton`).Scan(&retained, &current))
	require.NoError(t, tx.QueryRow(ctx, nativeReadinessCatalog45From50ProjectionSQL, accountBanTables, oid, original, catalogOID, originalCatalog).Scan(&reconstructed))
	require.Equal(t, retained, reconstructed, "substitution of only the exact old definition and qualified FK rows reconstructs genuine45")
	require.NotEqual(t, retained, current, "new50 cannot silently serve under retained45")
	require.Error(t, checkNativeReadinessCatalogVersion(ctx, tx, 45), "new50 observer is never a fixed45 body")
	require.NoError(t, tx.Rollback(ctx))
	cases := []struct{ name, sql string }{
		{"old body at50", nativeObserver45Definition},
		{"prior body at50", nativeObserverPrior50Definition},
		{"old catalog at50", nativeCatalog45Definition},
		{"catalog cost drift", `ALTER FUNCTION native_membership_guard.catalog() COST 101`},
		{"catalog ACL drift", `GRANT EXECUTE ON FUNCTION native_membership_guard.catalog() TO PUBLIC`},
		{"cost drift", `ALTER FUNCTION native_membership_guard.source_after() COST 101`},
		{"definer drift", `ALTER FUNCTION native_membership_guard.source_after() SECURITY INVOKER`},
		{"search path drift", `ALTER FUNCTION native_membership_guard.source_after() SET search_path=public,pg_catalog`},
		{"ACL drift", `GRANT EXECUTE ON FUNCTION native_membership_guard.source_after() TO PUBLIC`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
			require.NoError(t, err)
			defer tx.Rollback(ctx)
			_, err = tx.Exec(ctx, tc.sql)
			require.NoError(t, err, "the actual counterexample must be installed before asserting rejection")
			require.Error(t, checkNativeReadinessCatalogVersion(ctx, tx, 50))
		})
	}
}

// This exercises the original observation points, including another profile's
// owner-lease gap. The clock affects only the private PostgreSQL process; all
// source writes have the genuine same-backend fence and complete native frame.
func TestNativeObserverOtherProfileCommunityGap(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "explicit phase after genuine native50 source enrollment")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	setClock := func(value int64) {
		t.Helper()
		_, err := pool.Exec(ctx, fmt.Sprintf(`COPY (SELECT '%d') TO '/fixture-clock/walltime-us'`, value))
		require.NoError(t, err)
	}
	setClock(0)
	t.Cleanup(func() { setClock(0) })
	st := &SpaceStore{Pool: pool}
	rollback := errors.New("owned observer regression rollback")
	for _, gap := range []bool{true, false} {
		t.Run(map[bool]string{true: "time gap cannot preserve old history", false: "continuous unrelated write preserves history"}[gap], func(t *testing.T) {
			space, ordinary, community := nativeBanSpace(19), nativeBanProfile(3), nativeBanProfile(5)
			var finalErr error
			err := st.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
				var start time.Time
				require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&start))
				setClock(start.UnixMicro())
				capture, err := scoped.captureMemberMutationIfPresent(ctx, space, []uuid.UUID{ordinary})
				require.NoError(t, err)
				require.NotNil(t, capture)
				require.NotNil(t, capture.dbMutation)
				var witness string
				require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT old_sources->>'community' FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, community).Scan(&witness))
				require.NotEmpty(t, witness, "community-only B is included by full capture even though caller requested A")
				_, err = scoped.tx.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=$2 WHERE space_id=$1`, space, start.Add(time.Second))
				require.NoError(t, err, "shorten while owner support remains live")
				at := start.Add(500 * time.Millisecond)
				if gap {
					at = start.Add(2 * time.Second)
				}
				setClock(at.UnixMicro())
				_, err = scoped.tx.Exec(ctx, `UPDATE space_members SET joined_at=joined_at WHERE space_id=$1 AND profile_id=$2`, space, ordinary)
				require.NoError(t, err, "unrelated ordinary source callback must still observe old C witnesses")
				var cut bool
				require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT community_cut FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=$1 AND profile_id=$2`, space, community).Scan(&cut))
				require.Equal(t, gap, cut, "a profile-local optimization must not erase a sampled owner-lease gap")
				_, err = scoped.tx.Exec(ctx, `UPDATE community_owner_authority SET roster_lease_expires_at=$2 WHERE space_id=$1`, space, start.Add(10*time.Second))
				require.NoError(t, err, "same owner generation and community episode revive before original captured horizon")
				var restored string
				require.NoError(t, scoped.tx.QueryRow(ctx, `SELECT native_membership_guard.sources($1,$2,clock_timestamp())->>'community'`, space, community).Scan(&restored))
				require.Equal(t, witness, restored, "final witness equality alone cannot prove continuity")
				finalErr = scoped.finishEffectiveMemberMutations(ctx, capture)
				return rollback
			})
			require.ErrorIs(t, err, rollback)
			if gap {
				require.ErrorIs(t, finalErr, ErrEffectiveEpisodeUnavailable)
			} else {
				require.NoError(t, finalErr)
			}
			setClock(0)
		})
	}
}
