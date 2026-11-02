package sourcecustody

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

func TestControlInventoryPostgres(t *testing.T) {
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
			pool, err := pgxpool.New(ctx, dsn)
			require.NoError(t, err)
			defer pool.Close()
			_, err = pool.Exec(ctx, `CREATE ROLE old_controller LOGIN; CREATE ROLE new_owner NOLOGIN; CREATE ROLE new_controller NOLOGIN; CREATE ROLE new_responder NOLOGIN;
REVOKE CONNECT ON DATABASE control_fixture FROM PUBLIC;`+creationSchema+`
CREATE TABLE public.user_custody_state(singleton boolean PRIMARY KEY,state text NOT NULL,epoch bigint NOT NULL,source_id text NOT NULL,history_generation uuid NOT NULL,user_source_generation uuid NOT NULL,keyset_digest bytea NOT NULL);`)
			require.NoError(t, err)
			conn, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			inventory := ControlInventory{Admin: "upgrade_admin", Owner: "new_owner", Controller: "new_controller", Responder: "new_responder", LegacyLogins: []string{"old_controller"}}
			require.NoError(t, pool.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&inventory.ControlSystemID, &inventory.ControlDatabase, &inventory.ControlDatabaseOID))
			check := func() error { return CheckControlInventory(ctx, conn, inventory) }
			require.NoError(t, check(), "isolated explicit inventory should qualify")
			tx, err := conn.Begin(ctx)
			require.NoError(t, err)
			require.NoError(t, checkControlInventoryTx(ctx, tx, inventory))
			require.NoError(t, tx.Rollback(ctx))
			wrong := inventory
			wrong.ControlDatabaseOID++
			require.Error(t, CheckControlInventory(ctx, conn, wrong), "wrong physical database pin")
			wrong = inventory
			wrong.LegacyLogins = []string{inventory.Admin}
			require.Error(t, CheckControlInventory(ctx, conn, wrong), "identities must be distinct")
			for _, tc := range []struct{ name, add, undo string }{
				{"public table", `GRANT SELECT ON public.user_custody_state TO PUBLIC`, `REVOKE SELECT ON public.user_custody_state FROM PUBLIC`},
				{"public column", `GRANT UPDATE(state) ON public.user_custody_state TO PUBLIC`, `REVOKE UPDATE(state) ON public.user_custody_state FROM PUBLIC`},
				{"public connect", `GRANT CONNECT ON DATABASE control_fixture TO PUBLIC`, `REVOKE CONNECT ON DATABASE control_fixture FROM PUBLIC`},
				{"unknown login", `CREATE ROLE intruder LOGIN`, `DROP ROLE intruder`},
				{"membership", `GRANT new_owner TO old_controller`, `REVOKE new_owner FROM old_controller`},
				{"future login", `ALTER ROLE new_controller LOGIN`, `ALTER ROLE new_controller NOLOGIN`},
				{"future privilege", `GRANT SELECT ON public.user_custody_state TO new_responder`, `REVOKE SELECT ON public.user_custody_state FROM new_responder`},
				{"future owner", `ALTER TABLE public.user_custody_state OWNER TO new_controller`, `ALTER TABLE public.user_custody_state OWNER TO upgrade_admin`},
				{"lookalike temporary schema", `CREATE SCHEMA pgxtempy_owned AUTHORIZATION new_controller`, `DROP SCHEMA pgxtempy_owned`},
				{"database create", `GRANT CREATE ON DATABASE control_fixture TO new_controller`, `REVOKE CREATE ON DATABASE control_fixture FROM new_controller`},
				{"parameter grant", `GRANT SET ON PARAMETER session_replication_role TO new_controller`, `REVOKE SET ON PARAMETER session_replication_role FROM new_controller`},
				{"restricted function", `GRANT EXECUTE ON FUNCTION pg_catalog.pg_read_file(text) TO new_responder`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_read_file(text) FROM new_responder`},
				{"public restricted function", `GRANT EXECUTE ON FUNCTION pg_catalog.pg_read_file(text) TO PUBLIC`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_read_file(text) FROM PUBLIC`},
				{"security definer", `CREATE FUNCTION public.inventory_privileged() RETURNS int LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'`, `DROP FUNCTION public.inventory_privileged()`},
				{"system schema security definer", `CREATE FUNCTION pg_catalog.inventory_privileged() RETURNS int LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'`, `DROP FUNCTION pg_catalog.inventory_privileged()`},
				{"information schema security definer", `CREATE FUNCTION information_schema.inventory_privileged() RETURNS int LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'`, `DROP FUNCTION information_schema.inventory_privileged()`},
				{"missing table", `ALTER TABLE public.user_custody_state RENAME TO inventory_hidden`, `ALTER TABLE public.inventory_hidden RENAME TO user_custody_state`},
				{"indirect membership", `CREATE ROLE inventory_bridge NOLOGIN; GRANT inventory_bridge TO old_controller; GRANT new_owner TO inventory_bridge`, `REVOKE new_owner FROM inventory_bridge; REVOKE inventory_bridge FROM old_controller; DROP ROLE inventory_bridge`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					_, err := pool.Exec(ctx, tc.add)
					require.NoError(t, err)
					require.Error(t, check())
					_, err = pool.Exec(ctx, tc.undo)
					require.NoError(t, err)
					require.NoError(t, check())
				})
			}
			_, err = conn.Exec(ctx, `SET ROLE new_controller`)
			require.NoError(t, err)
			require.Error(t, check(), "current_user must equal independently supplied session_user")
			_, err = conn.Exec(ctx, `RESET ROLE`)
			require.NoError(t, err)
			require.NoError(t, check())
			_, err = pool.Exec(ctx, `CREATE ROLE lingering_login LOGIN PASSWORD 'fixture_only'; GRANT CONNECT ON DATABASE control_fixture TO lingering_login`)
			require.NoError(t, err)
			config := pool.Config().Copy()
			config.ConnConfig.User = "lingering_login"
			config.ConnConfig.Password = "fixture_only"
			lingering, err := pgxpool.NewWithConfig(ctx, config)
			require.NoError(t, err)
			require.NoError(t, lingering.Ping(ctx))
			_, err = pool.Exec(ctx, `ALTER ROLE lingering_login NOLOGIN`)
			require.NoError(t, err)
			require.Error(t, check(), "NOLOGIN leaves a previously authenticated session alive")
			lingering.Close()
			_, err = pool.Exec(ctx, `REVOKE CONNECT ON DATABASE control_fixture FROM lingering_login; DROP ROLE lingering_login`)
			require.NoError(t, err)
			require.NoError(t, check())
			_, err = pool.Exec(ctx, `BEGIN; SELECT 1; PREPARE TRANSACTION 'inventory_fixture'`)
			require.NoError(t, err)
			require.Error(t, check(), "prepared transaction must block")
			_, err = pool.Exec(ctx, `ROLLBACK PREPARED 'inventory_fixture'`)
			require.NoError(t, err)
			require.NoError(t, check())
		})
	}
}
