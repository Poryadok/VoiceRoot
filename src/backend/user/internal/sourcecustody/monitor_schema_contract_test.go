package sourcecustody

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

func TestMonitorSchemaContractPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	for _, version := range []string{"16-alpine", "18-alpine"} {
		t.Run(version, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			pg, err := postgres.Run(ctx, "postgres:"+version, postgres.BasicWaitStrategies(), postgres.WithDatabase("control_fixture"), postgres.WithUsername("schema_admin"), postgres.WithPassword("fixture_only"))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
			dsn, err := pg.ConnectionString(ctx, "sslmode=disable")
			require.NoError(t, err)
			pool, err := pgxpool.New(ctx, dsn)
			require.NoError(t, err)
			defer pool.Close()
			_, err = pool.Exec(ctx, `CREATE ROLE schema_owner NOLOGIN; CREATE ROLE schema_opener LOGIN PASSWORD 'fixture_only'; CREATE ROLE schema_controller LOGIN PASSWORD 'fixture_only'; CREATE ROLE schema_responder LOGIN PASSWORD 'fixture_only'; REVOKE CREATE ON SCHEMA public FROM PUBLIC;`+creationSchema+monitorSchemaDDL)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `ALTER TABLE public.user_custody_open_evidence OWNER TO schema_owner; ALTER TABLE public.user_custody_monitor_probe OWNER TO schema_owner; ALTER TABLE public.user_custody_monitor_response OWNER TO schema_owner; ALTER FUNCTION public.user_custody_monitor_probe_validate() OWNER TO schema_owner; REVOKE ALL ON FUNCTION public.user_custody_monitor_probe_validate() FROM PUBLIC; REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM PUBLIC; GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO schema_opener,schema_controller,schema_responder; GRANT USAGE ON SCHEMA public TO schema_opener,schema_controller,schema_responder; GRANT SELECT,INSERT ON public.user_custody_open_evidence TO schema_opener; GRANT SELECT ON public.user_custody_open_evidence TO schema_controller; GRANT SELECT,INSERT ON public.user_custody_monitor_probe TO schema_controller; GRANT SELECT ON public.user_custody_monitor_response TO schema_controller; GRANT SELECT ON public.user_custody_monitor_probe TO schema_responder; GRANT SELECT,INSERT ON public.user_custody_monitor_response TO schema_responder`)
			require.NoError(t, err)
			rolePool := func(name string) *pgxpool.Pool {
				cfg := pool.Config().Copy()
				cfg.ConnConfig.User = name
				cfg.ConnConfig.Password = "fixture_only"
				p, e := pgxpool.NewWithConfig(ctx, cfg)
				require.NoError(t, e)
				t.Cleanup(p.Close)
				require.NoError(t, p.Ping(ctx))
				return p
			}
			openerPool, controllerPool, responderPool := rolePool("schema_opener"), rolePool("schema_controller"), rolePool("schema_responder")
			admin, err := pool.Acquire(ctx)
			require.NoError(t, err)
			defer admin.Release()
			opener, err := openerPool.Acquire(ctx)
			require.NoError(t, err)
			defer opener.Release()
			controller, err := controllerPool.Acquire(ctx)
			require.NoError(t, err)
			defer controller.Release()
			responder, err := responderPool.Acquire(ctx)
			require.NoError(t, err)
			defer responder.Release()
			roles := monitorSchemaRoles{Admin: "schema_admin", Owner: "schema_owner", Opener: "schema_opener", Controller: "schema_controller", Responder: "schema_responder", SystemOwner: "schema_admin"}
			check := func() error { return qualifyMonitorSchema(ctx, admin, opener, controller, responder, roles) }
			require.NoError(t, check())
			_, err = admin.Exec(ctx, `CREATE TEMP TABLE pg_class(dummy int)`)
			require.NoError(t, err)
			require.NoError(t, check(), "temporary relation names cannot shadow catalog qualification")
			_, err = admin.Exec(ctx, `DROP TABLE pg_temp.pg_class`)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `GRANT SET ON PARAMETER session_replication_role TO schema_controller`)
			require.NoError(t, err)
			_, err = controller.Exec(ctx, `SET session_replication_role=replica`)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `REVOKE SET ON PARAMETER session_replication_role FROM schema_controller`)
			require.NoError(t, err)
			require.Error(t, check(), "a retained replica session must fail after SET privilege is revoked")
			_, err = pool.Exec(ctx, `GRANT SET ON PARAMETER session_replication_role TO schema_controller`)
			require.NoError(t, err)
			_, err = controller.Exec(ctx, `SET session_replication_role=origin`)
			require.NoError(t, err)
			_, err = pool.Exec(ctx, `REVOKE SET ON PARAMETER session_replication_role FROM schema_controller`)
			require.NoError(t, err)
			require.NoError(t, check())
			require.Error(t, qualifyMonitorSchema(ctx, admin, opener, responder, controller, roles), "authenticated roles must match their designated pools")
			_, err = pool.Exec(ctx, `CREATE DATABASE monitor_other_fixture`)
			require.NoError(t, err)
			otherCfg := responderPool.Config().Copy()
			otherCfg.ConnConfig.Database = "monitor_other_fixture"
			otherPool, err := pgxpool.NewWithConfig(ctx, otherCfg)
			require.NoError(t, err)
			other, err := otherPool.Acquire(ctx)
			require.NoError(t, err)
			require.Error(t, qualifyMonitorSchema(ctx, admin, opener, controller, other, roles), "sessions on a different database must fail")
			other.Release()
			otherPool.Close()
			for _, tc := range []struct{ name, add, undo string }{
				{"public grant", `GRANT SELECT ON public.user_custody_monitor_probe TO PUBLIC`, `REVOKE SELECT ON public.user_custody_monitor_probe FROM PUBLIC`},
				{"controller response insert", `GRANT INSERT ON public.user_custody_monitor_response TO schema_controller`, `REVOKE INSERT ON public.user_custody_monitor_response FROM schema_controller`},
				{"responder probe write", `GRANT INSERT ON public.user_custody_monitor_probe TO schema_responder`, `REVOKE INSERT ON public.user_custody_monitor_probe FROM schema_responder`},
				{"inherited role", `CREATE ROLE schema_bridge NOLOGIN; GRANT schema_bridge TO schema_responder`, `REVOKE schema_bridge FROM schema_responder; DROP ROLE schema_bridge`},
				{"missing trigger", `ALTER TABLE public.user_custody_monitor_probe DISABLE TRIGGER user_custody_monitor_probe_validate`, `ALTER TABLE public.user_custody_monitor_probe ENABLE TRIGGER user_custody_monitor_probe_validate`},
				{"disabled foreign key", `ALTER TABLE public.user_custody_monitor_probe DISABLE TRIGGER ALL; ALTER TABLE public.user_custody_monitor_probe ENABLE TRIGGER user_custody_monitor_probe_validate`, `ALTER TABLE public.user_custody_monitor_probe ENABLE TRIGGER ALL`},
				{"conditional trigger", `DROP TRIGGER user_custody_monitor_probe_validate ON public.user_custody_monitor_probe; CREATE TRIGGER user_custody_monitor_probe_validate BEFORE INSERT ON public.user_custody_monitor_probe FOR EACH ROW WHEN(false) EXECUTE FUNCTION public.user_custody_monitor_probe_validate()`, `DROP TRIGGER user_custody_monitor_probe_validate ON public.user_custody_monitor_probe; CREATE TRIGGER user_custody_monitor_probe_validate BEFORE INSERT ON public.user_custody_monitor_probe FOR EACH ROW EXECUTE FUNCTION public.user_custody_monitor_probe_validate()`},
				{"parameter privilege", `GRANT SET ON PARAMETER session_replication_role TO schema_controller`, `REVOKE SET ON PARAMETER session_replication_role FROM schema_controller`},
				{"role startup default", `ALTER ROLE schema_controller SET session_replication_role='replica'`, `ALTER ROLE schema_controller RESET session_replication_role`},
				{"database startup default", `ALTER DATABASE control_fixture SET session_replication_role='replica'`, `ALTER DATABASE control_fixture RESET session_replication_role`},
				{"role database startup default", `ALTER ROLE schema_responder IN DATABASE control_fixture SET session_replication_role='replica'`, `ALTER ROLE schema_responder IN DATABASE control_fixture RESET session_replication_role`},
				{"catalog security definer", `CREATE FUNCTION pg_catalog.monitor_escape_fixture() RETURNS int LANGUAGE sql SECURITY DEFINER AS 'SELECT 1'`, `DROP FUNCTION pg_catalog.monitor_escape_fixture()`},
				{"column grant", `GRANT UPDATE(nonce) ON public.user_custody_monitor_probe TO schema_responder`, `REVOKE UPDATE(nonce) ON public.user_custody_monitor_probe FROM schema_responder`},
				{"opener probe access", `GRANT SELECT ON public.user_custody_monitor_probe TO schema_opener`, `REVOKE SELECT ON public.user_custody_monitor_probe FROM schema_opener`},
				{"identity function PUBLIC", `GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO PUBLIC`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM PUBLIC`},
				{"identity function missing", `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM schema_responder`, `GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO schema_responder`},
				{"identity function extra", `CREATE ROLE schema_intruder NOLOGIN; GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO schema_intruder`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM schema_intruder; DROP ROLE schema_intruder`},
				{"opener identity missing", `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM schema_opener`, `GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO schema_opener`},
				{"weakened cap constraint", `ALTER TABLE public.user_custody_monitor_probe DROP CONSTRAINT monitor_probe_sequence_check; ALTER TABLE public.user_custody_monitor_probe ADD CONSTRAINT monitor_probe_sequence_check CHECK(sequence>0)`, `ALTER TABLE public.user_custody_monitor_probe DROP CONSTRAINT monitor_probe_sequence_check; ALTER TABLE public.user_custody_monitor_probe ADD CONSTRAINT monitor_probe_sequence_check CHECK(sequence BETWEEN 1 AND 86400)`},
				{"changed trigger function", `CREATE OR REPLACE FUNCTION public.user_custody_monitor_probe_validate() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $x$ BEGIN RETURN NEW; END $x$`, `CREATE OR REPLACE FUNCTION public.user_custody_monitor_probe_validate() RETURNS trigger LANGUAGE plpgsql SET search_path=pg_catalog AS $x$` + monitorProbeFunctionBody + `$x$`},
				{"changed function property", `ALTER FUNCTION public.user_custody_monitor_probe_validate() STABLE`, `ALTER FUNCTION public.user_custody_monitor_probe_validate() VOLATILE`},
				{"changed stop index", `DROP INDEX public.monitor_response_one_stop; CREATE UNIQUE INDEX monitor_response_one_stop ON public.user_custody_monitor_response(challenge) WHERE kind=2`, `DROP INDEX public.monitor_response_one_stop; CREATE UNIQUE INDEX monitor_response_one_stop ON public.user_custody_monitor_response(challenge) WHERE kind=3`},
				{"wrong owner", `ALTER TABLE public.user_custody_monitor_probe OWNER TO schema_controller`, `ALTER TABLE public.user_custody_monitor_probe OWNER TO schema_owner; GRANT SELECT,INSERT ON public.user_custody_monitor_probe TO schema_controller; GRANT SELECT ON public.user_custody_monitor_probe TO schema_responder`},
			} {
				t.Run(tc.name, func(t *testing.T) {
					_, e := pool.Exec(ctx, tc.add)
					require.NoError(t, e)
					require.Error(t, check())
					_, e = pool.Exec(ctx, tc.undo)
					require.NoError(t, e)
					require.NoError(t, check())
				})
			}
			challenge := make([]byte, 32)
			challenge[0] = 1
			nonce := make([]byte, 32)
			nonce[0] = 2
			payload := []byte("decision")
			digest := sha256.Sum256(payload)
			_, err = pool.Exec(ctx, `INSERT INTO public.user_custody_open_evidence(challenge,source_id,epoch,writer_role,payload,signature,key_digest) VALUES($1,'s',1,'writer',$2,decode(repeat('11',64),'hex'),decode(repeat('22',32),'hex'))`, challenge, payload)
			require.NoError(t, err)
			_, err = controller.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe(challenge,sequence,decision_digest,nonce) VALUES($1,1,$2,$3)`, challenge, make([]byte, 32), nonce)
			require.Error(t, err)
			_, err = controller.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe(challenge,sequence,decision_digest,nonce) VALUES($1,1,$2,$3)`, challenge, digest[:], nonce)
			require.NoError(t, err)
			_, err = responder.Exec(ctx, `INSERT INTO public.user_custody_monitor_response VALUES($1,1,1,$2,gen_random_uuid(),decode(repeat('33',64),'hex')),($1,1,3,$2,gen_random_uuid(),decode(repeat('44',64),'hex'))`, challenge, nonce)
			require.NoError(t, err)
			_, err = responder.Exec(ctx, `DELETE FROM public.user_custody_monitor_response WHERE kind=3`)
			require.Error(t, err)
			second := make([]byte, 32)
			second[0] = 4
			state := func(err error, expected string) {
				t.Helper()
				var pgerr *pgconn.PgError
				require.ErrorAs(t, err, &pgerr)
				require.Equal(t, expected, pgerr.Code)
			}
			_, err = opener.Exec(ctx, `INSERT INTO public.user_custody_open_evidence(challenge,source_id,epoch,writer_role,payload,signature,key_digest) VALUES($1,'s',1,'writer',$2,decode(repeat('11',64),'hex'),decode(repeat('22',32),'hex'))`, second, payload)
			require.NoError(t, err)
			badDigest := digest
			badDigest[0] ^= 1
			_, err = controller.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe VALUES($1,2,$2,$3)`, second, badDigest[:], nonce)
			state(err, "P0001")
			_, err = controller.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe VALUES($1,86401,$2,$3)`, second, digest[:], nonce)
			state(err, "23514")
			_, err = controller.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe VALUES($1,86400,$2,$3)`, second, digest[:], nonce)
			require.NoError(t, err)
			_, err = responder.Exec(ctx, `INSERT INTO public.user_custody_monitor_response VALUES($1,86400,2,$2,gen_random_uuid(),decode(repeat('33',64),'hex')),($1,86400,3,$2,gen_random_uuid(),decode(repeat('44',64),'hex'))`, second, nonce)
			require.NoError(t, err, "the terminal STOP has an independent lane at the cap")
			_, err = controller.Exec(ctx, `INSERT INTO public.user_custody_monitor_probe VALUES($1,86399,$2,$3)`, second, digest[:], nonce)
			require.NoError(t, err)
			wrongNonce := make([]byte, 32)
			wrongNonce[0] = 5
			_, err = responder.Exec(ctx, `INSERT INTO public.user_custody_monitor_response VALUES($1,86399,2,$2,gen_random_uuid(),decode(repeat('33',64),'hex'))`, second, wrongNonce)
			state(err, "23503")
			_, err = responder.Exec(ctx, `INSERT INTO public.user_custody_monitor_response VALUES($1,86399,3,$2,gen_random_uuid(),decode(repeat('44',64),'hex'))`, second, nonce)
			state(err, "23505")
		})
	}
}
