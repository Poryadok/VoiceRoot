package store

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// The pool's synchronous acquisition tracer blocks only the negative pre-DB
// path. No connection, native authority or successful SQL execution is faked.
type native51AcquireBarrier struct {
	entered chan struct{}
	exited  chan struct{}
}

func (b *native51AcquireBarrier) TraceAcquireStart(ctx context.Context, _ *pgxpool.Pool, _ pgxpool.TraceAcquireStartData) context.Context {
	close(b.entered)
	<-ctx.Done()
	close(b.exited)
	return ctx
}

func (*native51AcquireBarrier) TraceAcquireEnd(context.Context, *pgxpool.Pool, pgxpool.TraceAcquireEndData) {
}
func (*native51AcquireBarrier) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	return ctx
}
func (*native51AcquireBarrier) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func TestNativeCatalog51EngineTimeoutJoinsLockBeforeCleanup(t *testing.T) {
	for _, scenario := range []struct{ blockAt, cleanup string }{
		{"acquire", "outer-error"}, {"acquire", "engine-close"},
		{"window", "outer-error"}, {"window", "engine-close"},
	} {
		t.Run(scenario.blockAt+"/"+scenario.cleanup, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			barrier := &native51AcquireBarrier{entered: make(chan struct{}), exited: make(chan struct{})}
			cfg, err := pgxpool.ParseConfig("postgres://test@127.0.0.1:1/space_db?sslmode=disable")
			require.NoError(t, err)
			cfg.MinConns, cfg.MinIdleConns, cfg.MaxConns = 0, 0, 1
			cfg.ConnConfig.Tracer = barrier
			var dialCalls atomic.Int32
			cfg.ConnConfig.DialFunc = func(context.Context, string, string) (net.Conn, error) {
				dialCalls.Add(1)
				return nil, errors.New("network forbidden in timeout test")
			}
			pool, err := pgxpool.NewWithConfig(ctx, cfg)
			require.NoError(t, err)
			driver := &NativeCatalog51Migration{store: &SpaceStore{Pool: pool}, ctx: ctx, window: func(windowCtx context.Context) error {
				if scenario.blockAt == "window" {
					close(barrier.entered)
					<-windowCtx.Done()
					close(barrier.exited)
					return windowCtx.Err()
				}
				return nil
			}}
			t.Cleanup(func() {
				require.NoError(t, driver.Close())
				pool.Close()
			})
			sourceDir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(sourceDir, "000051_timeout.up.sql"), []byte("SELECT 1;"), 0600))
			sourceURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(sourceDir)}).String()
			engine, err := migrate.NewWithDatabaseInstance(sourceURL, "native51-test", driver)
			require.NoError(t, err)
			engine.LockTimeout = 100 * time.Millisecond
			result := make(chan error, 1)
			go func() { result <- engine.Migrate(51) }()
			select {
			case <-barrier.entered:
			case <-ctx.Done():
				t.Fatal("actual driver Lock never entered acquisition")
			}
			var engineErr error
			select {
			case engineErr = <-result:
			case <-ctx.Done():
				t.Fatal("pinned engine did not return its timeout")
			}
			require.ErrorIs(t, engineErr, migrate.ErrLockTimeout)
			select {
			case <-barrier.exited:
				t.Fatal("framework unexpectedly joined or canceled Driver.Lock")
			default:
			}
			if scenario.cleanup == "outer-error" {
				receipt, commitErr := driver.CommitQualified(engineErr)
				require.Nil(t, receipt)
				require.ErrorIs(t, commitErr, migrate.ErrLockTimeout)
			}
			sourceErr, closeErr := engine.Close()
			require.NoError(t, sourceErr)
			require.NoError(t, closeErr)
			select {
			case <-barrier.exited:
			default:
				t.Fatal("cleanup returned before owned acquisition exited")
			}
			require.Error(t, driver.Lock(), "a late framework goroutine cannot reopen a closed attempt")
			require.Nil(t, driver.conn)
			require.Nil(t, driver.tx)
			require.Nil(t, driver.receipt)
			require.EqualValues(t, 0, pool.Stat().AcquiredConns())
			require.Zero(t, dialCalls.Load(), "no real PG/network was used")
		})
	}
}

func TestNativeCatalog51CloseBeforeLockPreventsLateStart(t *testing.T) {
	driver := &NativeCatalog51Migration{ctx: context.Background()}
	require.NoError(t, driver.Close())
	require.Error(t, driver.Lock())
	require.Nil(t, driver.lockDone)
	require.Nil(t, driver.conn)
	require.Nil(t, driver.tx)
}

func TestNativeCatalog51PhaseRejectsIncompleteAndFailedEngine(t *testing.T) {
	for _, phase := range []nativeCatalog51Phase{catalog51New, catalog51Locked50, catalog51Dirty, catalog51Body} {
		state := nativeCatalog51State{phase: phase}
		require.Error(t, state.allowOuterCommit(nil))
	}
	state := nativeCatalog51State{phase: catalog51Qualified}
	require.NoError(t, state.allowOuterCommit(nil))
	engineFailure := errors.New("source reader failed outside database adapter")
	require.ErrorIs(t, state.allowOuterCommit(engineFailure), engineFailure)
	require.Error(t, state.allowOuterCommit(nil), "an earlier engine failure is irrevocable for this attempt")
}

func TestNativeCatalog51UnlockCannotAuthorizeCommit(t *testing.T) {
	state := nativeCatalog51State{phase: catalog51Qualified}
	state.frameworkUnlocked = true
	require.Error(t, state.allowOuterCommit(errors.New("engine unlock/source error")))
	state = nativeCatalog51State{phase: catalog51Body, frameworkUnlocked: true}
	require.Error(t, state.allowOuterCommit(nil), "unlock is not a qualified transition")
}
