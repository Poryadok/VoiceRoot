package sourcecustody

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"voice/backend/pkg/integrationtest"
)

type monitorTestAddr string

func (a monitorTestAddr) Network() string { return "tcp" }
func (a monitorTestAddr) String() string  { return string(a) }

func TestMonitorRemoteEndpointCanonicalization(t *testing.T) {
	for _, tc := range []struct {
		name, expected, actual string
		port                   uint16
		want                   bool
	}{
		{"ipv6 expanded", "::1", "[0:0:0:0:0:0:0:1]:5432", 5432, true},
		{"ipv4 mapped", "127.0.0.1", "[::ffff:127.0.0.1]:5432", 5432, true},
		{"different ipv6", "::1", "[::2]:5432", 5432, false},
		{"different port", "::1", "[::1]:5433", 5432, false},
		{"dns is not numeric", "::1", "localhost:5432", 5432, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, monitorRemoteMatches(monitorTestAddr(tc.actual), net.ParseIP(tc.expected), tc.port))
		})
	}
}

func TestMonitorConnectionGuardTLSPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("requires disposable TLS PostgreSQL")
	}
	integrationtest.ConfigureDockerTesting()
	for _, version := range []string{"16-alpine", "18-alpine"} {
		t.Run(version, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
			defer cancel()
			dir := t.TempDir()
			pki := processTLS(t, dir, "guard")
			configFile := filepath.Join(dir, "postgres.conf")
			require.NoError(t, os.WriteFile(configFile, []byte("listen_addresses='*'\nssl=on\nssl_ca_file='/tmp/testcontainers-go/postgres/ca_cert.pem'\nssl_cert_file='/tmp/testcontainers-go/postgres/server.cert'\nssl_key_file='/tmp/testcontainers-go/postgres/server.key'\n"), 0600))
			pg, err := postgres.Run(ctx, "postgres:"+version, postgres.BasicWaitStrategies(), postgres.WithDatabase("guard_fixture"), postgres.WithUsername("guard_admin"), postgres.WithPassword("fixture_only"), postgres.WithConfigFile(configFile), postgres.WithSSLCert(pki.caFile, pki.serverCert, pki.serverKey))
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, pg.Terminate(context.Background())) })
			raw, err := pg.ConnectionString(ctx)
			require.NoError(t, err)
			u, err := url.Parse(raw)
			require.NoError(t, err)
			_, port, err := net.SplitHostPort(u.Host)
			require.NoError(t, err)
			u.Host = net.JoinHostPort("127.0.0.1", port)
			q := u.Query()
			q.Set("sslmode", "verify-full")
			q.Set("sslrootcert", pki.caFile)
			u.RawQuery = q.Encode()
			admin, err := pgxpool.New(ctx, u.String())
			require.NoError(t, err)
			defer admin.Close()
			require.NoError(t, admin.Ping(ctx))
			_, err = admin.Exec(ctx, `CREATE ROLE guard_probe LOGIN PASSWORD 'fixture_only'; CREATE ROLE guard_other NOLOGIN; CREATE TABLE public.guard_fixture(value integer NOT NULL); GRANT SELECT,INSERT ON public.guard_fixture TO guard_probe; REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM PUBLIC; GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO guard_probe`)
			require.NoError(t, err)
			var system, database string
			var oid uint32
			require.NoError(t, admin.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()) FROM pg_control_system()`).Scan(&system, &database, &oid))
			u.User = url.UserPassword("guard_probe", "fixture_only")
			probeConfig, err := pinnedPostgresConfig(u.String())
			require.NoError(t, err)
			probeConfig.MaxConns = 1
			probe, err := pgxpool.NewWithConfig(ctx, probeConfig)
			require.NoError(t, err)
			defer probe.Close()
			require.NoError(t, probe.Ping(ctx))
			readers := []string{"guard_probe"}
			pin, err := newMonitorConnectionPin(u.String(), "guard_probe", system, database, oid, readers, "guard_admin", "127.0.0.1")
			require.NoError(t, err)
			_, err = newMonitorConnectionPin(u.String(), "guard_other", system, database, oid, readers, "guard_admin", "127.0.0.1")
			require.Error(t, err, "URL identity must match the pinned role")
			_, err = newMonitorConnectionPin(u.String(), "guard_probe", system, "other_database", oid, readers, "guard_admin", "127.0.0.1")
			require.Error(t, err, "URL database must match the physical pin")
			readers[0] = "guard_other"
			write := func(tx monitorGuardTx) error {
				if _, e := tx.Exec(ctx, `INSERT INTO public.guard_fixture VALUES(11)`); e != nil {
					return e
				}
				var value int
				return tx.QueryRow(ctx, `SELECT value FROM public.guard_fixture WHERE value=11`).Scan(&value)
			}
			require.NoError(t, withMonitorGuardTx(ctx, probe, pin, write))
			readers[0] = "guard_probe"
			plainURL := *u
			plainQuery := plainURL.Query()
			plainQuery.Set("sslmode", "disable")
			plainQuery.Del("sslrootcert")
			plainURL.RawQuery = plainQuery.Encode()
			plain, err := pgxpool.New(ctx, plainURL.String())
			require.NoError(t, err)
			require.NoError(t, plain.Ping(ctx))
			require.Error(t, withMonitorGuardTx(ctx, plain, pin, write), "plaintext is never an authenticated pinned endpoint")
			plain.Close()
			for _, grant := range []struct{ add, undo string }{
				{`GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO PUBLIC`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM PUBLIC`},
				{`REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM guard_probe`, `GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO guard_probe`},
				{`GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO guard_other`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM guard_other`},
				{`GRANT EXECUTE ON FUNCTION pg_catalog.pg_control_system() TO guard_probe WITH GRANT OPTION`, `REVOKE GRANT OPTION FOR EXECUTE ON FUNCTION pg_catalog.pg_control_system() FROM guard_probe`},
				{`GRANT EXECUTE ON FUNCTION pg_catalog.pg_read_file(text) TO guard_probe`, `REVOKE EXECUTE ON FUNCTION pg_catalog.pg_read_file(text) FROM guard_probe`},
				{`GRANT SET ON PARAMETER session_replication_role TO guard_probe`, `REVOKE SET ON PARAMETER session_replication_role FROM guard_probe`},
			} {
				_, err = admin.Exec(ctx, grant.add)
				require.NoError(t, err)
				require.Error(t, withMonitorGuardTx(ctx, probe, pin, write))
				_, err = admin.Exec(ctx, grant.undo)
				require.NoError(t, err)
				require.NoError(t, withMonitorGuardTx(ctx, probe, pin, write))
			}
			rolled := errors.New("fixture rollback")
			err = withMonitorGuardTx(ctx, probe, pin, func(tx monitorGuardTx) error {
				_, e := tx.Exec(ctx, `INSERT INTO public.guard_fixture VALUES(12)`)
				require.NoError(t, e)
				return rolled
			})
			require.Error(t, err)
			var n int
			require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM public.guard_fixture WHERE value=12`).Scan(&n))
			require.Zero(t, n)
			_, err = admin.Exec(ctx, `CREATE FUNCTION public.current_setting(text) RETURNS text LANGUAGE sql STABLE AS $$ SELECT CASE WHEN $1='search_path' THEN 'pg_catalog, pg_temp'::text ELSE pg_catalog.current_setting($1) END $$`)
			require.NoError(t, err)
			err = withMonitorGuardTx(ctx, probe, pin, func(tx monitorGuardTx) error {
				_, e := tx.Exec(ctx, `INSERT INTO public.guard_fixture VALUES(13); SET LOCAL search_path=public,pg_catalog`)
				return e
			})
			require.Error(t, err, "a shadowing function cannot conceal a changed search path")
			require.NoError(t, admin.QueryRow(ctx, `SELECT count(*) FROM public.guard_fixture WHERE value=13`).Scan(&n))
			require.Zero(t, n)
			_, err = admin.Exec(ctx, `DROP FUNCTION public.current_setting(text)`)
			require.NoError(t, err)
			wrong, err := newMonitorConnectionPin(u.String(), "guard_probe", "1", database, oid, readers, "guard_admin", "127.0.0.1")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write))
			wrong, err = newMonitorConnectionPin(u.String(), "guard_probe", system, database, oid+1, readers, "guard_admin", "127.0.0.1")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write))
			wrong, err = newMonitorConnectionPin(u.String(), "guard_probe", system, database, oid, readers, "guard_other", "127.0.0.1")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write), "identity function owner must match the independent trusted pin")
			wrongCA := processTLS(t, dir, "wrong")
			badURL := *u
			badQuery := badURL.Query()
			badQuery.Set("sslrootcert", wrongCA.caFile)
			badURL.RawQuery = badQuery.Encode()
			wrong, err = newMonitorConnectionPin(badURL.String(), "guard_probe", system, database, oid, readers, "guard_admin", "127.0.0.1")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write), "the actual TLS certificate must chain to the independently pinned CA")
			wrongHost := *u
			wrongHost.Host = net.JoinHostPort("localhost", port)
			wrong, err = newMonitorConnectionPin(wrongHost.String(), "guard_probe", system, database, oid, readers, "guard_admin", "127.0.0.1")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write), "a verified certificate does not override the pinned endpoint")
			wrong, err = newMonitorConnectionPin(u.String(), "guard_probe", system, database, oid, readers, "guard_admin", "127.0.0.2")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write), "actual remote IP must equal the independent pin")
			mapped, err := newMonitorConnectionPin(u.String(), "guard_probe", system, database, oid, readers, "guard_admin", "::ffff:127.0.0.1")
			require.NoError(t, err)
			require.NoError(t, withMonitorGuardTx(ctx, probe, mapped, write), "IPv4 and its canonical mapped form identify the same endpoint")
			portNum, err := strconv.Atoi(port)
			require.NoError(t, err)
			if portNum == 65535 {
				portNum--
			} else {
				portNum++
			}
			wrongPort := *u
			wrongPort.Host = net.JoinHostPort("127.0.0.1", strconv.Itoa(portNum))
			wrong, err = newMonitorConnectionPin(wrongPort.String(), "guard_probe", system, database, oid, readers, "guard_admin", "127.0.0.1")
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, wrong, write), "actual remote port must equal the independent pin")
			_, err = admin.Exec(ctx, `GRANT SET ON PARAMETER session_replication_role TO guard_probe`)
			require.NoError(t, err)
			conn, err := probe.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			unsafePID := conn.Conn().PgConn().PID()
			_, err = conn.Exec(ctx, `SET session_replication_role=replica`)
			require.NoError(t, err)
			conn.Release()
			_, err = admin.Exec(ctx, `REVOKE SET ON PARAMETER session_replication_role FROM guard_probe`)
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, pin, write), "retained replica connection must be discarded")
			conn, err = probe.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			require.NotEqual(t, unsafePID, conn.Conn().PgConn().PID(), "the poisoned backend must not be reused")
			conn.Release()
			require.NoError(t, withMonitorGuardTx(ctx, probe, pin, write), "a fresh origin connection may qualify")
			_, err = admin.Exec(ctx, `GRANT guard_other TO guard_probe`)
			require.NoError(t, err)
			conn, err = probe.Acquire(ctx)
			require.NoError(t, err)
			defer conn.Release()
			_, err = conn.Exec(ctx, `SET ROLE guard_other`)
			require.NoError(t, err)
			conn.Release()
			_, err = admin.Exec(ctx, `REVOKE guard_other FROM guard_probe`)
			require.NoError(t, err)
			require.Error(t, withMonitorGuardTx(ctx, probe, pin, write), "wrong current_user must be rejected")
			require.NoError(t, withMonitorGuardTx(ctx, probe, pin, write))
			require.Error(t, withMonitorGuardTx(ctx, probe, pin, func(tx monitorGuardTx) error { return tx.(pgx.Tx).Conn().Close(ctx) }), "uncertain transaction connection must not be reused")
			require.NoError(t, withMonitorGuardTx(ctx, probe, pin, write))
		})
	}
}
