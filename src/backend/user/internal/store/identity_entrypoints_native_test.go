package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func nativeIdentityEntryDB(t *testing.T) (*pgxpool.Pool, uuid.UUID, uuid.UUID) {
	t.Helper()
	dsn := os.Getenv("USER_ASSOCIATION_TEST_DSN")
	if dsn == "" {
		t.Skip("set USER_ASSOCIATION_TEST_DSN to owned PostgreSQL16")
	}
	ctx := context.Background()
	root, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	database := "entry_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	_, err = root.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	require.NoError(t, err)
	cfg, err := pgxpool.ParseConfig(dsn)
	require.NoError(t, err)
	cfg.ConnConfig.Database = database
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Close()
		_, err := root.Exec(ctx, "DROP DATABASE "+pgx.Identifier{database}.Sanitize())
		require.NoError(t, err)
		root.Close()
	})
	_, err = pool.Exec(ctx, `CREATE TABLE schema_migrations(version bigint NOT NULL,dirty boolean NOT NULL); INSERT INTO schema_migrations VALUES(0,false)`)
	require.NoError(t, err)
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok)
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "src", "backend", "migrations", "user_db")); err == nil {
			break
		}
		next := filepath.Dir(dir)
		require.NotEqual(t, dir, next)
		dir = next
	}
	migrations := filepath.Join(dir, "src", "backend", "migrations", "user_db")
	files, err := os.ReadDir(migrations)
	require.NoError(t, err)
	profile, account := uuid.New(), uuid.New()
	for _, file := range files {
		if !strings.HasSuffix(file.Name(), ".up.sql") || file.Name() >= "000023" {
			continue
		}
		version, err := strconv.Atoi(file.Name()[:6])
		require.NoError(t, err)
		if version == 22 {
			_, err = pool.Exec(ctx, `INSERT INTO profiles(id,account_id,username,discriminator,display_name) VALUES($1,$2,'entryfixture','1234','Entry fixture')`, profile, account)
			require.NoError(t, err)
		}
		body, err := os.ReadFile(filepath.Join(migrations, file.Name()))
		require.NoError(t, err)
		_, err = pool.Exec(ctx, string(body))
		require.NoError(t, err, file.Name())
		_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version=$1,dirty=false`, version)
		require.NoError(t, err)
	}
	return pool, profile, account
}

func TestNativeMixedWritersAcquireIdentityBeforeSubordinateLocks(t *testing.T) {
	for _, name := range []string{"sdk", "verification", "organization-start", "organization-complete", "search-baseline", "search-backfill", "account-regular", "notification-preferences", "free-tier-freeze"} {
		t.Run(name, func(t *testing.T) {
			pool, profile, account := nativeIdentityEntryDB(t)
			store := NewProfileStore(pool)
			ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
			defer cancel()
			var operation func() error
			switch name {
			case "sdk":
				operation = func() error {
					_, err := store.RecordSdkAuthorTombstone(ctx, SdkAuthorTombstoneInput{OperationID: uuid.New(), SourceAccountID: uuid.New(), SourceActorID: uuid.New(), TargetAccountID: account, TargetProfileID: profile, ExpectedProfileRevision: 1, FrozenBindingID: uuid.New(), FrozenAuthorityEpoch: 1, FreezeReceiptID: uuid.New(), RequestHash: strings.Repeat("a", 64)})
					return err
				}
			case "verification":
				operation = func() error {
					_, _, err := store.ApplyVerificationSourceState(ctx, profile, "organization_dns", 1, true, "organization", "dns")
					return err
				}
			case "organization-start":
				operation = func() error { _, err := store.StartOrgVerification(ctx, profile, "example.test"); return err }
			case "organization-complete":
				_, err := store.StartOrgVerification(ctx, profile, "example.test")
				require.NoError(t, err)
				_, token, err := store.LatestOrgVerification(ctx, profile)
				require.NoError(t, err)
				operation = func() error { _, err := store.MarkOrgVerificationVerified(ctx, profile, token); return err }
			case "search-baseline":
				operation = func() error { return store.MaterializeSearchProjectionBaseline(ctx) }
			case "search-backfill":
				operation = func() error { _, err := store.BackfillSearchKeys(ctx, 1); return err }
			case "account-regular":
				operation = func() error { return store.MarkAccountRegular(ctx, account) }
			case "notification-preferences":
				operation = func() error { return store.SetNotificationPrefsJSON(ctx, profile, `{}`) }
			case "free-tier-freeze":
				operation = func() error { return store.ApplyFreeTierFreeze(ctx, account) }
			}
			owner, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer owner.Rollback(context.Background())
			_, err = owner.Exec(ctx, `SELECT singleton FROM identity_meta WHERE singleton FOR UPDATE`)
			require.NoError(t, err)
			returned := make(chan error, 1)
			go func() { returned <- operation() }()
			bounded, stop := context.WithTimeout(ctx, time.Second)
			defer stop()
			for {
				select {
				case err := <-returned:
					t.Fatalf("writer returned before I-first wait: %v", err)
				default:
				}
				var blocked bool
				err = pool.QueryRow(bounded, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE $1::integer=ANY(pg_blocking_pids(pid)) AND query LIKE '%identity_meta%')`, owner.Conn().PgConn().PID()).Scan(&blocked)
				require.NoError(t, err)
				if blocked {
					break
				}
				select {
				case <-bounded.Done():
					t.Fatal("writer did not wait on I")
				case <-time.After(5 * time.Millisecond):
				}
			}
			// A separate real connection proves the waiter owns no profile/allocator
			// lock before I, rather than inferring lock order from elapsed duration.
			witness, err := pool.Begin(ctx)
			require.NoError(t, err)
			defer witness.Rollback(context.Background())
			_, err = witness.Exec(ctx, `SELECT id FROM profiles WHERE id=$1 FOR UPDATE NOWAIT`, profile)
			require.NoError(t, err)
			_, err = witness.Exec(ctx, `SELECT singleton FROM user_profile_search_offset WHERE singleton FOR UPDATE NOWAIT`)
			require.NoError(t, err)
			require.NoError(t, witness.Rollback(ctx))
			var checkpoints int
			require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM profile_search_key_backfill_checkpoints`).Scan(&checkpoints))
			require.Zero(t, checkpoints, "backfill cannot insert/lock checkpoint before I")
			require.NoError(t, owner.Commit(ctx))
			require.NoError(t, <-returned)
		})
	}
}
