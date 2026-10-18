package partysnapshot

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type nativeCutFunc func(context.Context, pgx.Tx, Subject) (Candidate, error)

func (f nativeCutFunc) ReadNativeCut(ctx context.Context, tx pgx.Tx, subject Subject) (Candidate, error) {
	return f(ctx, tx, subject)
}

type floorFunc func(context.Context, string) (int64, error)

func (f floorFunc) CurrentFloor(ctx context.Context, account string) (int64, error) {
	return f(ctx, account)
}

func postgresFixture(t *testing.T) (*pgxpool.Pool, string) {
	t.Helper()
	dsn := os.Getenv("BE011_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("real PostgreSQL unavailable: set BE011_TEST_PG_DSN")
	}
	ctx := context.Background()
	config, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	config.MaxConns = 3
	pool, err := pgxpool.NewWithConfig(ctx, config)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	table := "party_cut_" + uuid.New().String()[:8]
	_, err = pool.Exec(ctx, `CREATE TABLE `+table+` (revision BIGINT NOT NULL); INSERT INTO `+table+` VALUES (1)`)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DROP TABLE `+table); pool.Close() })
	return pool, table
}

func TestPostgresReaderPinsOneReadOnlyCut(t *testing.T) {
	pool, table := postgresFixture(t)
	ctx := context.Background()
	var first, again int64
	var isolation, readOnly string
	reader := &PostgresReader{Database: pool, Source: nativeCutFunc(func(ctx context.Context, tx pgx.Tx, _ Subject) (Candidate, error) {
		require.NoError(t, tx.QueryRow(ctx, `SHOW transaction_isolation`).Scan(&isolation))
		require.NoError(t, tx.QueryRow(ctx, `SHOW transaction_read_only`).Scan(&readOnly))
		require.NoError(t, tx.QueryRow(ctx, `SELECT revision FROM `+table).Scan(&first))
		_, err := pool.Exec(ctx, `UPDATE `+table+` SET revision=2`)
		require.NoError(t, err)
		require.NoError(t, tx.QueryRow(ctx, `SELECT revision FROM `+table).Scan(&again))
		return Candidate{DatabaseTime: time.Unix(1, 0)}, nil
	})}
	before := time.Now().Add(-time.Second)
	cut, err := reader.Read(ctx, testSubject())
	require.NoError(t, err)
	require.Equal(t, "repeatable read", isolation)
	require.Equal(t, "on", readOnly)
	require.Equal(t, int64(1), first)
	require.Equal(t, first, again, "concurrent committed writer must not mix the cut")
	require.True(t, cut.DatabaseTime.After(before), "replace source timestamp with a final DB clock sample")
	require.Zero(t, pool.Stat().AcquiredConns())
}

func TestPostgresReaderRejectsWritesAndReleasesFailedTransaction(t *testing.T) {
	pool, table := postgresFixture(t)
	reader := &PostgresReader{Database: pool, Source: nativeCutFunc(func(ctx context.Context, tx pgx.Tx, _ Subject) (Candidate, error) {
		_, err := tx.Exec(ctx, `UPDATE `+table+` SET revision=9`)
		require.Error(t, err, "native reader must be database-enforced read only")
		return Candidate{}, err
	})}
	_, err := reader.Read(context.Background(), testSubject())
	require.ErrorIs(t, err, ErrUnavailable)
	require.Zero(t, pool.Stat().AcquiredConns())
	var revision int64
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT revision FROM `+table).Scan(&revision))
	require.Equal(t, int64(1), revision)
}

func TestPostgresTwoPassRetriesConcurrentMutationWithFreshAuth(t *testing.T) {
	pool, table := postgresFixture(t)
	nativeReads, authReads := 0, 0
	fixture := testRoster()
	reader := &PostgresReader{Database: pool, Source: nativeCutFunc(func(ctx context.Context, tx pgx.Tx, _ Subject) (Candidate, error) {
		nativeReads++
		cut := fixture
		var revision int64
		if err := tx.QueryRow(ctx, `SELECT revision FROM `+table).Scan(&revision); err != nil {
			return Candidate{}, err
		}
		cut.RosterVersion = uint64(revision)
		cut.Fingerprint[0] = byte(revision)
		return cut, nil
	})}
	engine := &Engine{Reader: reader, Normalizer: &recordedNormalizer{}, Floors: floorFunc(func(ctx context.Context, _ string) (int64, error) {
		authReads++
		require.Zero(t, pool.Stat().AcquiredConns(), "Auth must never run inside native read transaction")
		if authReads == 1 {
			_, err := pool.Exec(ctx, `UPDATE `+table+` SET revision=2`)
			require.NoError(t, err)
		}
		return 7, nil
	})}
	result, err := engine.Snapshot(context.Background(), testSubject())
	require.NoError(t, err)
	require.Equal(t, uint64(2), result.RosterVersion)
	require.Equal(t, 4, nativeReads)
	require.Equal(t, 2, authReads)
	require.Zero(t, pool.Stat().AcquiredConns())
}
