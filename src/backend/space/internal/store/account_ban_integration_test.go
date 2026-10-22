//go:build spacebanintegration

package store

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
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
	space, owner, actor, account := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO spaces(id,name,owner_profile_id) VALUES($1,'complete capture required',$2)`, space, owner)
	require.NoError(t, err)
	// A missing User capture dependency must never be interpreted as a complete
	// empty cohort, even when the account currently has no selected profile hint.
	err = (&SpaceStore{Pool: pool}).BanMember(ctx, space, account, actor, nil, nil)
	require.Error(t, err, "unqualified owner/cohort cannot authorize an account ban")
	var bans, audit int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM space_bans WHERE space_id=$1`, space).Scan(&bans))
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE space_id=$1 AND action='member_banned'`, space).Scan(&audit))
	require.Zero(t, bans)
	require.Zero(t, audit)
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
