package partysearch

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"voice/backend/matchmaking/internal/store"
)

func TestPlatformBanGuardRejectsAnyMemberAndAcceptsRevocationAndExpiry(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	bans := &store.BanStore{Pool: pool}
	accounts := []uuid.UUID{uuid.New(), uuid.New()}
	check := func(want error) {
		t.Helper()
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(ctx) }()
		guard, err := store.AcquirePlatformBanGuard(ctx, tx, accounts)
		require.NoError(t, err)
		require.ErrorIs(t, guard.Check(ctx), want)
	}
	check(nil)
	require.NoError(t, bans.InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: accounts[1], BannedByProfileID: uuid.New()}))
	check(store.ErrPlatformBanDenied)
	require.NoError(t, bans.RevokePlatformMMBan(ctx, accounts[1]))
	check(nil)
	expired := time.Now().Add(-time.Minute)
	require.NoError(t, bans.InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: accounts[1], BannedByProfileID: uuid.New(), ExpiresAt: &expired}))
	check(nil)
}

func TestPlatformBanGuardRequiresReadCommittedAndValidAccounts(t *testing.T) {
	pool := partyDB(t)
	ctx := context.Background()
	for _, accounts := range [][]uuid.UUID{nil, {uuid.Nil}} {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()
		_, err = store.AcquirePlatformBanGuard(ctx, tx, accounts)
		require.ErrorIs(t, err, store.ErrPlatformBanUnavailable)
		require.NoError(t, tx.Rollback(ctx))
	}
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	require.NoError(t, err)
	defer func() { _ = tx.Rollback(ctx) }()
	_, err = store.AcquirePlatformBanGuard(ctx, tx, []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, store.ErrPlatformBanUnavailable)
	_, err = store.AcquirePlatformBanGuard(ctx, nil, []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, store.ErrPlatformBanUnavailable)
	var missing *store.PlatformBanGuard
	require.ErrorIs(t, missing.Check(ctx), store.ErrPlatformBanUnavailable)
}

func waitForBanLock(t *testing.T, ctx context.Context, pool *pgxpool.Pool, query string) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event='advisory' AND query LIKE $1)`, query).Scan(&waiting)
		return err == nil && waiting
	}, 2*time.Second, 10*time.Millisecond)
}

func TestPlatformBanGuardReadsCommittedBanAfterWaitingForSanctionWriter(t *testing.T) {
	pool := partyDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	account := uuid.New()
	_, err := pool.Exec(ctx, `CREATE FUNCTION pause_test_platform_ban() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN PERFORM pg_advisory_xact_lock(145007); RETURN NEW; END $$; CREATE TRIGGER pause_platform_ban BEFORE INSERT ON mm_platform_bans FOR EACH ROW EXECUTE FUNCTION pause_test_platform_ban()`)
	require.NoError(t, err)
	blocker, err := pool.Begin(ctx)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback(context.Background()) }()
	_, err = blocker.Exec(ctx, `SELECT pg_advisory_xact_lock(145007)`)
	require.NoError(t, err)
	reader, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	require.NoError(t, err)
	defer func() { _ = reader.Rollback(context.Background()) }()
	var before int
	require.NoError(t, reader.QueryRow(ctx, `SELECT count(*) FROM mm_platform_bans`).Scan(&before))
	require.Zero(t, before)
	written := make(chan error, 1)
	go func() {
		written <- (&store.BanStore{Pool: pool}).InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: account, BannedByProfileID: uuid.New()})
	}()
	waitForBanLock(t, ctx, pool, "%INSERT INTO mm_platform_bans%")
	type result struct {
		guard *store.PlatformBanGuard
		err   error
	}
	acquired := make(chan result, 1)
	go func() {
		guard, err := store.AcquirePlatformBanGuard(ctx, reader, []uuid.UUID{account})
		acquired <- result{guard, err}
	}()
	waitForBanLock(t, ctx, pool, "%pg_advisory_xact_lock_shared%")
	require.NoError(t, blocker.Commit(ctx))
	require.NoError(t, <-written)
	got := <-acquired
	require.NoError(t, got.err)
	require.ErrorIs(t, got.guard.Check(ctx), store.ErrPlatformBanDenied)
}

func TestPlatformSanctionWritersWaitForGuardTransaction(t *testing.T) {
	pool := partyDB(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	account := uuid.New()
	bans := &store.BanStore{Pool: pool}
	for _, write := range []func() error{
		func() error {
			return bans.InsertPlatformMMBan(ctx, store.InsertPlatformMMBanParams{AccountID: account, BannedByProfileID: uuid.New()})
		},
		func() error { return bans.RevokePlatformMMBan(ctx, account) },
	} {
		tx, err := pool.Begin(ctx)
		require.NoError(t, err)
		defer func() { _ = tx.Rollback(context.Background()) }()
		guard, err := store.AcquirePlatformBanGuard(ctx, tx, []uuid.UUID{account, account})
		require.NoError(t, err)
		require.NoError(t, guard.Check(ctx))
		written := make(chan error, 1)
		go func() { written <- write() }()
		waitForBanLock(t, ctx, pool, "%pg_advisory_xact_lock(%")
		require.NoError(t, tx.Commit(ctx))
		require.NoError(t, <-written)
		// Make the next transaction eligible while still testing the revoke writer.
		_, err = pool.Exec(ctx, `UPDATE mm_platform_bans SET expires_at=clock_timestamp()-interval '1 second' WHERE account_id=$1`, account)
		require.NoError(t, err)
	}
}
