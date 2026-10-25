//go:build spacebanintegration

package store

import (
	"context"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	userv1 "voice.app/voice/user/v1"
)

// Explicit phase after TestAccountBanNative50 in the owned source-built clock
// image. The fixed tmpfs file affects only PostgreSQL process wall time through
// testdata/account_ban_wall_clock.c. No immutable journal timestamp is edited;
// no kernel/host clock, Go monotonic time, public API or production DDL changes.
func TestAccountBanFixtureClockExpiry(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned source-bound clock fixture required")
	owner, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(owner.Close)
	setClock := func(micros int64) {
		t.Helper()
		_, err := owner.Exec(ctx, fmt.Sprintf(`COPY (SELECT '%d') TO '/fixture-clock/walltime-us'`, micros))
		require.NoError(t, err, "fixed private tmpfs test clock only")
	}
	setClock(0)
	t.Cleanup(func() { setClock(0) })
	serving := projectionServingLogin(t, ctx, owner)
	space, operation := nativeBanSpace(7), uuid.New()
	var captures atomic.Int32
	st := &SpaceStore{Pool: serving, AccountCaptures: nativeBanCaptureFixture(func(_ context.Context, r *userv1.CaptureSpaceMemberAccountBindingsRequest) (*userv1.CaptureSpaceMemberAccountBindingsResponse, error) {
		captures.Add(1)
		return nativeBanOwnerFixture(r), nil
	})}
	require.NoError(t, nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error { return nil }))
	var completed time.Time
	require.NoError(t, serving.QueryRow(ctx, `SELECT completed_at FROM space_account_ban_operations WHERE actor_profile_id=$1 AND client_operation_id=$2 AND state='COMMITTED'`, nativeBanProfile(1), operation).Scan(&completed))
	var otherRoots, capturing int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE space_id<>$1`, space).Scan(&otherRoots))
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE state='CAPTURING'`).Scan(&capturing))
	require.Positive(t, capturing, "actual incomplete/reconciliation phase must remain present")
	boundary := completed.Add(30 * 24 * time.Hour).UnixMicro()
	setClock(boundary)
	var actual int64
	require.NoError(t, owner.QueryRow(ctx, `SELECT (extract(epoch FROM clock_timestamp())*1000000)::bigint`).Scan(&actual))
	require.Equal(t, boundary, actual, "the source-built process clock must be effective")
	err = nativeBanExecute(t, st, space, nativeBanAccount(2), nativeBanProfile(1), operation, nil, func(context.Context) error {
		t.Error("retained expired intent must fail before new Role/effects")
		return nil
	})
	require.Equal(t, codes.FailedPrecondition, status.Code(err))
	require.Empty(t, status.Convert(err).Details(), "expiry is not INTENT_IN_PROGRESS")
	require.EqualValues(t, 1, captures.Load())
	removed, err := st.CleanupExpiredAccountBanIntents(ctx, space, 256)
	require.NoError(t, err)
	require.Zero(t, removed, "strictly greater than30days, equality cannot delete")
	setClock(boundary + 1)
	spaces, err := st.ListExpiredAccountBanIntentSpaces(ctx, 256)
	require.NoError(t, err)
	require.Contains(t, spaces, space)
	removed, err = st.CleanupExpiredAccountBanIntents(ctx, space, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	var count int
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_captures WHERE space_id=$1`, space).Scan(&count))
	require.Zero(t, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE space_id<>$1`, space).Scan(&count))
	require.Equal(t, otherRoots, count)
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE state='CAPTURING'`).Scan(&count))
	require.Equal(t, capturing, count, "CAPTURING age is not a completed-outcome retention policy")
	require.NoError(t, serving.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&count))
	require.Equal(t, 1, count, "cleanup neither renews outcomes nor erases the domain audit")
}
