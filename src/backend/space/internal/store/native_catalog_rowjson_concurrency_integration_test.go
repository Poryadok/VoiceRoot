//go:build spacebanintegration

package store

import (
	"context"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	migrateengine "github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// Only the transport error is injected. All history hooks, actual migration SQL,
// source checks, parent reconstruction and CAS use the real production adapter.
type native51FailingDriver struct {
	database.Driver
	at string
}

var errNative51Injected = errors.New("native51 injected transaction boundary failure")

func (d native51FailingDriver) Run(r io.Reader) error {
	if err := d.Driver.Run(r); err != nil {
		return err
	}
	if d.at == "after body" {
		return errNative51Injected
	}
	return nil
}

func (d native51FailingDriver) SetVersion(version int, dirty bool) error {
	if err := d.Driver.SetVersion(version, dirty); err != nil {
		return err
	}
	if (dirty && d.at == "after dirty marker") || (!dirty && d.at == "after CAS and clean marker") {
		return errNative51Injected
	}
	return nil
}

func (d native51FailingDriver) Unlock() error {
	if err := d.Driver.Unlock(); err != nil {
		return err
	}
	if d.at == "engine unlock error" {
		return errNative51Injected
	}
	return nil
}

type native51CommittedSnapshot struct {
	version    int64
	dirty      bool
	definition string
	epoch      string
	seal       []byte
	catalog    []byte
}

func native51Snapshot(t *testing.T, ctx context.Context, pool *pgxpool.Pool) native51CommittedSnapshot {
	t.Helper()
	var s native51CommittedSnapshot
	require.NoError(t, pool.QueryRow(ctx, `SELECT m.version,m.dirty,pg_get_functiondef('native_membership_guard.catalog()'::regprocedure),e.epoch::text,e.catalog_sha256,native_membership_guard.catalog() FROM public.schema_migrations m CROSS JOIN native_membership_guard.epoch e WHERE e.singleton`).Scan(&s.version, &s.dirty, &s.definition, &s.epoch, &s.seal, &s.catalog))
	return s
}

func TestNative51AtomicMigrationFailureRollsBackBodySealAndHistory(t *testing.T) {
	ctx, pool := native51Old50SourceFixture(t)
	require.NoError(t, requireNativeCatalogRowJSONAdmission(), "this transition test requires separately qualified candidate stability; never bypass the admission gate")
	st := &SpaceStore{Pool: pool}
	prior, err := st.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	before := native51Snapshot(t, ctx, pool)
	dir, err := filepath.Abs(filepath.Join(repoRoot(t), "src/backend/migrations/space_db"))
	require.NoError(t, err)
	for _, at := range []string{"after dirty marker", "after body", "after CAS and clean marker", "engine unlock error", "outer window failure"} {
		t.Run(at, func(t *testing.T) {
			windowLost := false
			driver, err := st.NewNativeCatalog51Migration(ctx, prior, func(context.Context) error {
				if windowLost {
					return errNative51Injected
				}
				return nil
			})
			require.NoError(t, err)
			defer func() { require.NoError(t, driver.Close()) }()
			engine, err := migrateengine.NewWithDatabaseInstance((&url.URL{Scheme: "file", Path: filepath.ToSlash(dir)}).String(), "native51-boundary-test", native51FailingDriver{Driver: driver, at: at})
			require.NoError(t, err)
			engineErr := engine.Migrate(51)
			if at == "outer window failure" {
				require.NoError(t, engineErr)
				windowLost = true
			} else {
				require.ErrorIs(t, engineErr, errNative51Injected)
			}
			receipt, commitErr := driver.CommitQualified(engineErr)
			require.Error(t, commitErr)
			require.Nil(t, receipt)
			sourceErr, closeErr := engine.Close()
			require.NoError(t, sourceErr)
			require.NoError(t, closeErr)
			require.Equal(t, before, native51Snapshot(t, ctx, pool), "actual DDL, epoch and history must all revert together: "+at)
		})
	}
}

func TestNative51FrameworkUnlockRetainsTransactionFences(t *testing.T) {
	ctx, pool := native51Old50SourceFixture(t)
	require.NoError(t, requireNativeCatalogRowJSONAdmission())
	st := &SpaceStore{Pool: pool}
	prior, err := st.InspectNativeInstallerSource(ctx, 50)
	require.NoError(t, err)
	driver, err := st.NewNativeCatalog51Migration(ctx, prior, func(context.Context) error { return nil })
	require.NoError(t, err)
	defer func() { require.NoError(t, driver.Close()) }()
	require.NoError(t, driver.Lock())
	other, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer other.Release()
	assertBlocked := func() {
		t.Helper()
		var acquired bool
		require.NoError(t, other.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, driver.key).Scan(&acquired))
		require.False(t, acquired, "standard migration key remains fenced by the SAME backend transaction")
		tx, err := other.Begin(ctx)
		require.NoError(t, err)
		_, err = tx.Exec(ctx, `LOCK TABLE public.schema_migrations IN ACCESS SHARE MODE NOWAIT`)
		require.ErrorContains(t, err, "could not obtain lock")
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		require.NoError(t, tx.Rollback(cleanup))
	}
	assertBlocked()
	require.NoError(t, driver.Unlock())
	assertBlocked()
	require.NoError(t, driver.Close())
	var acquired bool
	require.NoError(t, other.QueryRow(ctx, `SELECT pg_try_advisory_lock($1::bigint)`, driver.key).Scan(&acquired))
	require.True(t, acquired)
	require.NoError(t, other.QueryRow(ctx, `SELECT pg_advisory_unlock($1::bigint)`, driver.key).Scan(&acquired))
	require.True(t, acquired)
}
