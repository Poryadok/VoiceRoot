package sourcecustody

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// monitorGuardTx exposes no transaction completion methods. The callback must
// be trusted fixed internal code: it must not retain or concurrently use the
// transaction, execute transaction-control SQL, or commit it by type assertion.
// This is not an untrusted SQL sandbox.
type monitorGuardTx interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}

// monitorConnectionPin is not an assertion of exclusive custody or source
// continuity. Its identityReaders must be the independently selected complete
// list, not inferred from observed grants. PostgreSQL 16 can retain a former
// superuser's authentication-time authority despite current role demotion; a
// stopped/restarted control boundary and fresh credentials remain mandatory.
// Endpoint IP changes require an independently qualified pin update and fail
// closed until then. IP and TLS checks do not establish clone/restore custody.
// This private guard is not wired into production.
type monitorConnectionPin struct {
	role, system, database, host string
	identityFunctionOwner        string
	oid                          uint32
	port                         uint16
	roots                        *x509.CertPool
	remoteIP                     net.IP
	identityReaders              []string
}

func newMonitorConnectionPin(raw, role, system, database string, oid uint32, identityReaders []string, identityFunctionOwner, remoteIP string) (*monitorConnectionPin, error) {
	if forbiddenPGEnvironment() || !validControlURL(raw) || !databaseName.MatchString(role) || !databaseName.MatchString(identityFunctionOwner) || !systemID.MatchString(system) || !databaseName.MatchString(database) || oid == 0 {
		return nil, ErrUnavailable
	}
	if len(identityReaders) == 0 || len(identityReaders) > 3 {
		return nil, ErrUnavailable
	}
	seen := map[string]bool{}
	for _, name := range identityReaders {
		if !databaseName.MatchString(name) || seen[name] {
			return nil, ErrUnavailable
		}
		seen[name] = true
	}
	if !seen[role] {
		return nil, ErrUnavailable
	}
	ip := net.ParseIP(remoteIP)
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() {
		return nil, ErrUnavailable
	}
	cfg, err := pinnedPostgresConfig(raw)
	if err != nil || cfg.ConnConfig.User != role || cfg.ConnConfig.Database != database {
		return nil, ErrUnavailable
	}
	c := cfg.ConnConfig
	return &monitorConnectionPin{role: role, system: system, database: database, host: c.Host, port: c.Port, oid: oid, roots: c.TLSConfig.RootCAs.Clone(), identityReaders: append([]string(nil), identityReaders...), identityFunctionOwner: identityFunctionOwner, remoteIP: append(net.IP(nil), ip...)}, nil
}

// withMonitorGuardTx checks this acquired connection before and after a fixed
// internal operation and commits its insert/readback together. It provides no
// ongoing schema, stop-boundary, host or clone/restore custody guarantees.
// A failed or cancelled COMMIT has an unknown outcome, not guaranteed rollback.
func withMonitorGuardTx(ctx context.Context, pool *pgxpool.Pool, pin *monitorConnectionPin, fn func(monitorGuardTx) error) error {
	if ctx == nil || ctx.Err() != nil || pool == nil || pin == nil || fn == nil || forbiddenPGEnvironment() {
		return ErrUnavailable
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	discard := true
	defer func() {
		if discard {
			c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = conn.Conn().Close(c)
			cancel()
		}
		conn.Release()
	}()
	if !monitorGuardTLS(conn, pin) {
		return ErrUnavailable
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if e := tx.Rollback(c); e != nil && !errors.Is(e, pgx.ErrTxClosed) {
			_ = conn.Conn().Close(c)
		}
	}()
	if _, err = tx.Exec(ctx, `SET LOCAL search_path=pg_catalog,pg_temp`); err != nil || checkMonitorGuardTx(ctx, tx, pin) != nil {
		return ErrUnavailable
	}
	if err = fn(tx); err != nil || ctx.Err() != nil || checkMonitorGuardTx(ctx, tx, pin) != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	if tx.Commit(ctx) != nil || ctx.Err() != nil {
		return ErrUnavailable
	}
	discard = false
	return nil
}

func monitorGuardTLS(conn *pgxpool.Conn, pin *monitorConnectionPin) bool {
	if conn == nil || pin == nil || pin.roots == nil {
		return false
	}
	pc := conn.Conn()
	cfg := pc.Config()
	if cfg.Host != pin.host || cfg.Port != pin.port || cfg.Database != pin.database || cfg.User != pin.role || len(cfg.Fallbacks) != 0 || len(cfg.RuntimeParams) != 0 || cfg.TLSConfig == nil || cfg.TLSConfig.InsecureSkipVerify || cfg.TLSConfig.ServerName != pin.host || cfg.TLSConfig.RootCAs == nil {
		return false
	}
	tlsConn, ok := pc.PgConn().Conn().(*tls.Conn)
	if !ok {
		return false
	}
	state := tlsConn.ConnectionState()
	if !state.HandshakeComplete || len(state.VerifiedChains) == 0 || len(state.PeerCertificates) == 0 {
		return false
	}
	intermediates := x509.NewCertPool()
	for _, cert := range state.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	if _, err := state.PeerCertificates[0].Verify(x509.VerifyOptions{Roots: pin.roots, Intermediates: intermediates, DNSName: pin.host, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}); err != nil {
		return false
	}
	return monitorRemoteMatches(tlsConn.RemoteAddr(), pin.remoteIP, pin.port)
}

func monitorRemoteMatches(addr net.Addr, expected net.IP, port uint16) bool {
	if addr == nil || expected == nil {
		return false
	}
	host, actualPort, err := net.SplitHostPort(addr.String())
	actual := net.ParseIP(host)
	return err == nil && actual != nil && expected.Equal(actual) && actualPort == strconv.Itoa(int(port))
}

func checkMonitorGuardTx(ctx context.Context, tx pgx.Tx, pin *monitorConnectionPin) error {
	// Do not resolve any unqualified predicate until the callback's path has
	// been checked using a catalog-qualified function.
	var path string
	if tx.QueryRow(ctx, `SELECT pg_catalog.current_setting('search_path')`).Scan(&path) != nil || path != "pg_catalog, pg_temp" {
		return ErrUnavailable
	}
	var valid bool
	if tx.QueryRow(ctx, `SELECT session_user=$1 AND current_user=$1 AND current_setting('session_replication_role')='origin' AND current_setting('search_path')='pg_catalog, pg_temp' AND current_setting('transaction_isolation')='read committed'
 AND current_database()=$2 AND (SELECT oid FROM pg_database WHERE datname=current_database())=$3::oid AND (SELECT system_identifier::text FROM pg_catalog.pg_control_system())=$4
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1 AND rolcanlogin AND NOT (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls))
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members m JOIN pg_roles a ON a.oid=m.member JOIN pg_roles b ON b.oid=m.roleid WHERE a.rolname=$1 OR b.rolname=$1)
 AND has_function_privilege($1,'pg_catalog.pg_control_system()','EXECUTE')
 AND EXISTS(SELECT 1 FROM pg_proc p JOIN pg_roles o ON o.oid=p.proowner JOIN pg_language l ON l.oid=p.prolang WHERE p.oid='pg_catalog.pg_control_system()'::regprocedure AND o.rolname=$7 AND o.rolsuper AND l.lanname='internal' AND p.prosrc='pg_control_system' AND NOT p.prosecdef)
 AND (SELECT count(*) FROM pg_roles WHERE rolname=ANY($5::text[]))=cardinality($5::text[])
 AND NOT EXISTS(SELECT 1 FROM unnest($5::text[]) r WHERE NOT has_function_privilege(r,'pg_catalog.pg_control_system()','EXECUTE'))
 AND NOT EXISTS(SELECT 1 FROM pg_proc p CROSS JOIN LATERAL aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE p.oid='pg_catalog.pg_control_system()'::regprocedure AND (a.grantee=0 OR (a.grantee<>p.proowner AND (pg_get_userbyid(a.grantee)<>ALL($5::text[]) OR a.is_grantable))))
 AND NOT EXISTS(SELECT 1 FROM pg_settings s WHERE (s.context IN ('superuser','superuser-backend') AND has_parameter_privilege($1,s.name,'SET')) OR has_parameter_privilege($1,s.name,'ALTER SYSTEM'))
 AND NOT EXISTS(SELECT 1 FROM pg_database d WHERE has_database_privilege($1,d.oid,'CREATE'))
 AND NOT EXISTS(SELECT 1 FROM pg_namespace n WHERE n.oid<>pg_my_temp_schema() AND NOT pg_is_other_temp_schema(n.oid) AND has_schema_privilege($1,n.oid,'CREATE'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_class c ON c.relowner=r.oid WHERE r.rolname=$1)
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON p.proowner=r.oid WHERE r.rolname=$1)
 AND NOT EXISTS(SELECT 1 FROM pg_proc p WHERE p.prosecdef AND has_function_privilege($1,p.oid,'EXECUTE'))
 AND NOT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='pg_catalog' AND p.oid<>'pg_catalog.pg_control_system()'::regprocedure AND has_function_privilege($1,p.oid,'EXECUTE') AND (p.proname=ANY($6::text[]) OR NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')))`, pin.role, pin.database, pin.oid, pin.system, pin.identityReaders, []string{"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file", "pg_ls_logdir", "pg_ls_waldir", "pg_ls_archive_statusdir", "pg_ls_tmpdir", "pg_ls_replslotdir", "pg_ls_logicalmapdir", "pg_ls_logicalsnapdir", "lo_import", "lo_export", "pg_reload_conf", "pg_rotate_logfile", "pg_promote", "pg_create_restore_point", "pg_switch_wal", "pg_backup_start", "pg_backup_stop"}, pin.identityFunctionOwner).Scan(&valid) != nil || !valid {
		return ErrUnavailable
	}
	return nil
}
