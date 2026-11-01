package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

// This tests PostgreSQL lock behavior only; it does not exercise an upgrade or
// prove the source is fenced. Production fencing still needs its own tests.
func TestControlCatalogLocksAcrossDatabases(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	for _, version := range []string{"16-alpine", "18-alpine"} {
		t.Run(version, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			pg, err := postgres.Run(ctx, "postgres:"+version, postgres.BasicWaitStrategies(), postgres.WithDatabase("control_fixture"), postgres.WithUsername("upgrade_admin"), postgres.WithPassword("fixture_only"), testcontainers.WithConfigModifier(func(cfg *container.Config) { cfg.Cmd = []string{"postgres", "-c", "max_prepared_transactions=10"} }))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
			dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
			require.NoError(t, err)
			control, err := pgxpool.New(ctx, dsn)
			require.NoError(t, err)
			defer control.Close()
			_, err = control.Exec(ctx, `CREATE DATABASE other_fixture`)
			require.NoError(t, err)
			_, err = control.Exec(ctx, `CREATE ROLE role_fixture LOGIN; CREATE TABLE public.control_fixture_lock(id int)`)
			require.NoError(t, err)
			config := control.Config().Copy()
			config.ConnConfig.Database = "other_fixture"
			other, err := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, err)
			defer other.Close()
			require.NoError(t, other.Ping(ctx))

			// Role changes in another database can be prepared and are visible
			// cluster-wide; their shared-catalog lock prevents the fence lock.
			_, err = other.Exec(ctx, `BEGIN; ALTER ROLE role_fixture NOLOGIN; PREPARE TRANSACTION 'cross_db_role_fixture'`)
			require.NoError(t, err)
			var prepared int
			require.NoError(t, control.QueryRow(ctx, `SELECT count(*) FROM pg_prepared_xacts WHERE database='other_fixture' AND gid='cross_db_role_fixture'`).Scan(&prepared))
			require.Equal(t, 1, prepared)
			tx, err := control.Begin(ctx)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `SET LOCAL lock_timeout='250ms'`)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `LOCK TABLE pg_catalog.pg_authid, pg_catalog.pg_auth_members IN ACCESS EXCLUSIVE MODE`)
			var pgerr *pgconn.PgError
			require.ErrorAs(t, err, &pgerr)
			require.Equal(t, "55P03", pgerr.Code)
			require.NoError(t, tx.Rollback(ctx))
			_, err = other.Exec(ctx, `ROLLBACK PREPARED 'cross_db_role_fixture'`)
			require.NoError(t, err)
			_, err = control.Exec(ctx, `CREATE ROLE member_fixture NOLOGIN`)
			require.NoError(t, err)
			_, err = other.Exec(ctx, `BEGIN; GRANT role_fixture TO member_fixture; PREPARE TRANSACTION 'cross_db_membership_fixture'`)
			require.NoError(t, err)
			tx, err = control.Begin(ctx)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `SET LOCAL lock_timeout='250ms'`)
			require.NoError(t, err)
			_, err = tx.Exec(ctx, `LOCK TABLE pg_catalog.pg_authid, pg_catalog.pg_auth_members IN ACCESS EXCLUSIVE MODE`)
			pgerr = nil
			require.ErrorAs(t, err, &pgerr)
			require.Equal(t, "55P03", pgerr.Code)
			require.NoError(t, tx.Rollback(ctx))
			_, err = other.Exec(ctx, `ROLLBACK PREPARED 'cross_db_membership_fixture'`)
			require.NoError(t, err)

			// Both upgrade contenders take shared catalogs before the control
			// table. A second database's ALTER ROLE must also wait.
			observer, err := control.Acquire(ctx)
			require.NoError(t, err)
			defer observer.Release()
			otherConn, err := other.Acquire(ctx)
			require.NoError(t, err)
			defer otherConn.Release()
			second, err := control.Begin(ctx)
			require.NoError(t, err)
			defer second.Rollback(context.Background())
			waitingPIDs := []int32{int32(otherConn.Conn().PgConn().PID()), int32(second.Conn().PgConn().PID())}
			tx, err = control.Begin(ctx)
			require.NoError(t, err)
			defer tx.Rollback(context.Background())
			_, err = tx.Exec(ctx, `LOCK TABLE pg_catalog.pg_authid, pg_catalog.pg_auth_members IN ACCESS EXCLUSIVE MODE; LOCK TABLE public.control_fixture_lock IN ACCESS EXCLUSIVE MODE`)
			require.NoError(t, err)
			result := make(chan error, 1)
			go func() { _, changeErr := otherConn.Exec(ctx, `ALTER ROLE role_fixture NOLOGIN`); result <- changeErr }()
			ordered := make(chan error, 1)
			go func() {
				_, lockErr := second.Exec(ctx, `LOCK TABLE pg_catalog.pg_authid, pg_catalog.pg_auth_members IN ACCESS EXCLUSIVE MODE; LOCK TABLE public.control_fixture_lock IN ACCESS EXCLUSIVE MODE`)
				if lockErr == nil {
					lockErr = second.Commit(ctx)
				}
				ordered <- lockErr
			}()
			require.Eventually(t, func() bool {
				var waiting int
				return observer.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='relation' AND relation='pg_catalog.pg_authid'::regclass AND NOT granted AND pid=ANY($1::int[])`, waitingPIDs).Scan(&waiting) == nil && waiting == 2
			}, 5*time.Second, 25*time.Millisecond, "both statements must actually wait on the shared catalog")
			select {
			case err = <-result:
				t.Fatalf("role change completed while catalog lock held: %v", err)
			case <-time.After(300 * time.Millisecond):
			}
			select {
			case err = <-ordered:
				t.Fatalf("second ordered lock completed while first held: %v", err)
			case <-time.After(300 * time.Millisecond):
			}
			require.NoError(t, tx.Commit(ctx))
			select {
			case err = <-result:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			select {
			case err = <-ordered:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}

			// An already authenticated legacy superuser must not retain a
			// pre-lock authorization after the role fence commits.
			_, err = control.Exec(ctx, `CREATE ROLE legacy_super LOGIN SUPERUSER PASSWORD 'fixture_only'`)
			require.NoError(t, err)
			legacyConfig := control.Config().Copy()
			legacyConfig.ConnConfig.User = "legacy_super"
			legacyConfig.ConnConfig.Password = "fixture_only"
			legacy, err := pgxpool.NewWithConfig(ctx, legacyConfig)
			require.NoError(t, err)
			defer legacy.Close()
			legacyConn, err := legacy.Acquire(ctx)
			require.NoError(t, err)
			defer legacyConn.Release()
			legacyPID := legacyConn.Conn().PgConn().PID()
			fence, err := control.Begin(ctx)
			require.NoError(t, err)
			defer fence.Rollback(context.Background())
			_, err = fence.Exec(ctx, `LOCK TABLE pg_catalog.pg_authid, pg_catalog.pg_auth_members IN ACCESS EXCLUSIVE MODE; ALTER ROLE legacy_super NOLOGIN NOSUPERUSER`)
			require.NoError(t, err)
			oldResult := make(chan error, 1)
			go func() { _, oldErr := legacyConn.Exec(ctx, `ALTER ROLE role_fixture SUPERUSER`); oldResult <- oldErr }()
			require.Eventually(t, func() bool {
				var waiting int
				return observer.QueryRow(ctx, `SELECT count(*) FROM pg_locks WHERE locktype='relation' AND relation='pg_catalog.pg_authid'::regclass AND NOT granted AND pid=$1`, legacyPID).Scan(&waiting) == nil && waiting == 1
			}, 5*time.Second, 25*time.Millisecond)
			require.NoError(t, fence.Commit(ctx))
			select {
			case err = <-oldResult:
				var denial *pgconn.PgError
				require.ErrorAs(t, err, &denial, "legacy superuser statement must not survive revocation")
				require.Equal(t, "42501", denial.Code)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			// On PG16 authentication-time superuser authority survives ALTER
			// ROLE; PG18 denies this particular escalation after demotion.
			// Neither observation proves that retained sessions are fenced.
			_, err = legacyConn.Exec(ctx, `SET SESSION AUTHORIZATION upgrade_admin`)
			if version == "16-alpine" {
				require.NoError(t, err)
				var sessionUser string
				require.NoError(t, legacyConn.QueryRow(ctx, `SELECT session_user`).Scan(&sessionUser))
				require.Equal(t, "upgrade_admin", sessionUser)
			} else {
				var denial *pgconn.PgError
				require.ErrorAs(t, err, &denial)
				require.Equal(t, "42501", denial.Code)
			}
		})
	}
}
