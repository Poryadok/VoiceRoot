//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	userv1 "voice.app/voice/user/v1"
)

// Explicit historical component input before native heads/guard installation;
// no LEGACY0 is synthesized/backfilled through a serving path or privilege grant.
func TestAccountBanFixtureLargeCohort34(t *testing.T) {
	nativeBanFixtureLegacyCohort34(t, 301)
}

func TestAccountBanFixtureOver100Cohort34(t *testing.T) {
	nativeBanFixtureLegacyCohort34(t, 101)
}

func nativeBanFixtureLegacyCohort34(t *testing.T, extra int) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	var version int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version))
	require.EqualValues(t, 34, version)
	tag, err := pool.Exec(ctx, `INSERT INTO space_members(space_id,profile_id)
	 SELECT $1,format('10000000-0000-0000-0000-%s',lpad(to_hex(n),12,'0'))::uuid FROM generate_series(20,$2::integer) n`, nativeBanSpace(13), 19+extra)
	require.NoError(t, err)
	require.EqualValues(t, extra, tag.RowsAffected())
	_, err = pool.Exec(ctx, `UPDATE spaces SET member_count=(SELECT count(*) FROM
 (SELECT profile_id FROM space_members WHERE space_id=$1 UNION SELECT profile_id FROM community_roster_members WHERE space_id=$1) cohort) WHERE id=$1`, nativeBanSpace(13))
	require.NoError(t, err)
}

func TestAccountBanLargeCohort50(t *testing.T) {
	nativeBanCheckCohort50(t, 307, 304, true)
}

func TestAccountBanOver100Cohort50(t *testing.T) {
	nativeBanCheckCohort50(t, 107, 104, true)
}

func TestAccountBanOver100Retry50(t *testing.T) {
	nativeBanCheckCohort50(t, 107, 104, false)
}

func nativeBanCheckCohort50(t *testing.T, entries, matched int, initialize bool) {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn)
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	space := nativeBanSpace(13)
	admin := &SpaceStore{Pool: owner}
	if initialize {
		require.NoError(t, admin.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
			return scoped.InitializeScopedNativeMembership(ctx, space)
		}))
	}
	serving := projectionServingLogin(t, ctx, owner)
	if !initialize {
		// The previous failed Ban really committed administrative enrollment.
		// Readiness is checked by the actual serving login, never the installer.
		read := &SpaceStore{Pool: serving}
		require.NoError(t, read.withOwnershipScope(ctx, []uuid.UUID{space}, func(scoped *SpaceStore) error {
			return scoped.RequireScopedNativeMembershipReadiness(ctx, space)
		}))
	}
	started := time.Now()
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		t.Logf("owner capture entered after %s", time.Since(started))
		require.Len(t, r.Cohort.Entries, entries, "no paginated first100/fullclaim limit")
		return nativeBanOwnerFixture(r), nil
	})}
	operation := uuid.New()
	if !initialize {
		require.NoError(t, serving.QueryRow(ctx, `SELECT client_operation_id FROM space_account_ban_operations WHERE space_id=$1 AND state='CAPTURING'`, space).Scan(&operation), "retry exact failed logical intent")
	}
	err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error { return nil })
	t.Logf("Ban phase elapsed=%s error=%v", time.Since(started), err)
	if err != nil {
		nativeBanAssertNoEffect(t, serving, space)
		var qualifications int
		require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_qualifications WHERE space_id=$1`, space).Scan(&qualifications))
		require.Zero(t, qualifications, "failed phase cannot publish partial identity facts")
	}
	require.NoError(t, err)
	var count int
	require.NoError(t, serving.QueryRow(ctx, `SELECT cardinality(matched_profiles) FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2 AND state='COMMITTED'`, nativeBanProfile(1), operation).Scan(&count))
	require.Equal(t, matched, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT member_count FROM spaces WHERE id=$1`, space).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_members WHERE space_id=$1`, space).Scan(&count))
	require.Equal(t, 2, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_community_member_heads WHERE space_id=$1 AND state='ACTIVE'`, space).Scan(&count))
	require.Zero(t, count)
}
