//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	spacev1 "voice.app/voice/space/v1"
)

// This suite requires a fresh, owned database migrated with the real driver.
// It intentionally has no Testcontainers fallback and cannot silently skip.
func TestAccountBanCannotSucceedWithoutCompleteOwnerCapture(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned PostgreSQL fixture required")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	facts, err := (&SpaceStore{Pool: pool}).InspectNativeInstallerSource(ctx, 51)
	require.NoError(t, err)
	require.NotNil(t, facts)
	require.EqualValues(t, 51, facts.Version)
	serving := projectionServingLogin(t, ctx, pool)
	st := &SpaceStore{Pool: serving}
	create, _, actor := signedProjectionCreator(t, st, &spacev1.CreateSpaceRequest{Name: "complete capture required", Visibility: "private"})
	row, err := create()
	require.NoError(t, err)
	require.NotNil(t, row)
	space, account := row.ID, uuid.New()
	// A missing User capture dependency must never be interpreted as a complete
	// empty cohort, even when the account currently has no selected profile hint.
	authorized := false
	err = nativeBanExecute(t, st, space, account, actor, uuid.New(), nil, func(context.Context) error {
		authorized = true
		return nil
	})
	require.ErrorIs(t, err, ErrAccountBanUnavailable, "missing User capture must fail closed")
	require.False(t, authorized, "missing capture is rejected before authorization and intent claim")
	var bans, audit, operations int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1`, space).Scan(&bans))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&audit))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_account_ban_operations WHERE space_id=$1`, space).Scan(&operations))
	require.Zero(t, bans)
	require.Zero(t, audit)
	require.Zero(t, operations)
}

func TestAccountBanFixtureTrusted45Transition(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned PostgreSQL fixture required")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	receipt, err := (&SpaceStore{Pool: pool}).TransitionNativeMembershipCatalog45(ctx)
	require.NoError(t, err, "actual trusted45 transition; no marker or catalog forgery")
	require.NotNil(t, receipt)
	t.Logf("trusted45 database=%s migration=%s changed=%t", receipt.DatabaseName, receipt.MigrationSHA256, receipt.Changed)
}

func TestAccountBanFixtureTrusted50Transition(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("SPACE_BAN_TEST_DATABASE_URL")
	require.NotEmpty(t, dsn, "owned PostgreSQL fixture required")
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	s := &SpaceStore{Pool: pool}
	var before, current []byte
	require.NoError(t, pool.QueryRow(ctx, `SELECT catalog_sha256,native_membership_guard.catalog() FROM native_membership_guard.epoch WHERE singleton`).Scan(&before, &current))
	require.NotEqual(t, before, current, "fresh real M50 must retain the trusted45 seal until this explicit transition")
	receipt, err := s.TransitionNativeMembershipCatalog50(ctx)
	require.NoError(t, err)
	require.True(t, receipt.Changed)
	require.NotEqual(t, receipt.OldSealSHA256, receipt.NewSealSHA256)
	require.Equal(t, "d52240e13e70eead705db61cf462cefc860957d40f7f52bb0bf40962c73dc3f3", receipt.MigrationSHA256)
	repeat, err := s.TransitionNativeMembershipCatalog50(ctx)
	require.NoError(t, err)
	require.False(t, repeat.Changed)
	require.Equal(t, receipt.OldSealSHA256, repeat.OldSealSHA256)
	require.Equal(t, receipt.NewSealSHA256, repeat.NewSealSHA256)
	t.Logf("trusted50 database=%s migration=%s old=%s new=%s changed=%t repeat=%t", receipt.DatabaseName, receipt.MigrationSHA256, receipt.OldSealSHA256, receipt.NewSealSHA256, receipt.Changed, repeat.Changed)
}
