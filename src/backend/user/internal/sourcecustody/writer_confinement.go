package sourcecustody

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CheckWriterConfinement checks database-local role and ownership constraints
// on one source session. It does not establish deployed binary identity,
// network confinement, exclusive administrator custody or restore continuity.
func CheckWriterConfinement(ctx context.Context, pool *pgxpool.Pool, expected PhysicalSource, writer string) error {
	return checkWriterConfinement(ctx, pool, expected, writer, true)
}

// CheckDormantWriterConfinement checks the same role restrictions while LOGIN
// is disabled for a prospective cutover. Disabling LOGIN does not terminate
// existing sessions; the caller must separately fence and drain them.
func CheckDormantWriterConfinement(ctx context.Context, pool *pgxpool.Pool, expected PhysicalSource, writer string) error {
	return checkWriterConfinement(ctx, pool, expected, writer, false)
}

func checkWriterConfinement(ctx context.Context, pool *pgxpool.Pool, expected PhysicalSource, writer string, expectLogin bool) error {
	if ctx == nil || ctx.Err() != nil || pool == nil || !systemID.MatchString(expected.SystemIdentifier) || !databaseName.MatchString(expected.Database) || expected.DatabaseOID == 0 || !databaseName.MatchString(writer) {
		return ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return ErrUnavailable
	}
	defer conn.Release()
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 2*time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if checkWriterConfinementTx(ctx, tx, expected, writer, expectLogin) != nil || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}

func checkWriterConfinementTx(ctx context.Context, tx pgx.Tx, expected PhysicalSource, writer string, expectLogin bool) error {
	if ctx == nil || tx == nil || !systemID.MatchString(expected.SystemIdentifier) || !databaseName.MatchString(expected.Database) || expected.DatabaseOID == 0 || !databaseName.MatchString(writer) {
		return ErrUnavailable
	}
	var actual PhysicalSource
	var administrator bool
	if tx.QueryRow(ctx, `SELECT system_identifier::text,current_database(),(SELECT oid FROM pg_database WHERE datname=current_database()),(SELECT rolsuper FROM pg_roles WHERE rolname=current_user) FROM pg_control_system()`).Scan(&actual.SystemIdentifier, &actual.Database, &actual.DatabaseOID, &administrator) != nil || actual != expected || !administrator {
		return ErrUnavailable
	}
	var roleOID uint32
	var login, isAdmin bool
	if tx.QueryRow(ctx, `SELECT oid,rolcanlogin,rolname=current_user FROM pg_roles WHERE rolname=$1`, writer).Scan(&roleOID, &login, &isAdmin) != nil || roleOID == 0 || login != expectLogin || isAdmin {
		return ErrUnavailable
	}
	var unsafe, publicConnect, unexpectedLogin, usable bool
	if tx.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM pg_roles r WHERE pg_has_role($1::oid,r.oid,'MEMBER') AND
		(r.rolsuper OR r.rolcreaterole OR r.rolcreatedb OR r.rolreplication OR r.rolbypassrls OR r.rolname=ANY($2::text[])))`, roleOID,
		[]string{"pg_read_server_files", "pg_write_server_files", "pg_execute_server_program", "pg_signal_backend", "pg_read_all_data", "pg_write_all_data", "pg_checkpoint", "pg_create_subscription", "pg_maintain", "pg_database_owner"}).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_settings s WHERE (s.context IN ('superuser','superuser-backend') AND has_parameter_privilege($1::oid,s.name,'SET')) OR has_parameter_privilege($1::oid,s.name,'ALTER SYSTEM'))`, roleOID).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='pg_catalog' AND p.proname=ANY($2::text[]) AND has_function_privilege($1::oid,p.oid,'EXECUTE'))`, roleOID,
		[]string{"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file", "pg_ls_logdir", "pg_ls_waldir", "pg_ls_archive_statusdir", "pg_ls_tmpdir", "pg_ls_replslotdir", "pg_ls_logicalmapdir", "pg_ls_logicalsnapdir", "lo_import", "lo_export", "pg_reload_conf", "pg_rotate_logfile", "pg_promote", "pg_create_restore_point", "pg_switch_wal", "pg_backup_start", "pg_backup_stop"}).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	// A direct function grant can bypass predefined-role membership checks.
	// Permit the one required physical-identity function, but reject other
	// restricted pg_catalog functions made executable for this writer.
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='pg_catalog' AND p.oid<>'pg_catalog.pg_control_system()'::regprocedure AND has_function_privilege($1::oid,p.oid,'EXECUTE') AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'))`, roleOID).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database d CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) a WHERE d.oid=$1 AND a.grantee=0 AND a.privilege_type='CONNECT')`, expected.DatabaseOID).Scan(&publicConnect) != nil || publicConnect {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles r WHERE r.rolcanlogin AND NOT r.rolsuper AND r.oid<>$1 AND r.rolname<>current_user AND (has_database_privilege(r.oid,$2::oid,'CONNECT') OR pg_has_role(r.oid,$1::oid,'MEMBER')))`, roleOID, expected.DatabaseOID).Scan(&unexpectedLogin) != nil || unexpectedLogin {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database d WHERE d.datallowconn AND d.oid<>$2 AND has_database_privilege($1::oid,d.oid,'CONNECT'))`, roleOID, expected.DatabaseOID).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT has_database_privilege($1::oid,$2::oid,'CONNECT') AND has_database_privilege($1::oid,$2::oid,'TEMP') AND NOT has_database_privilege($1::oid,$2::oid,'CREATE') AND has_schema_privilege($1::oid,'public','USAGE') AND NOT has_schema_privilege($1::oid,'public','CREATE') AND has_function_privilege($1::oid,'pg_catalog.pg_control_system()','EXECUTE') AND to_regclass('public.identity_meta') IS NOT NULL AND to_regclass('public.profiles') IS NOT NULL`, roleOID, expected.DatabaseOID).Scan(&usable) != nil || !usable {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_database d WHERE d.oid=$2 AND pg_has_role($1::oid,d.datdba,'MEMBER')) OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname !~ '^pg_(temp|toast_temp)_[0-9]+$' AND pg_has_role($1::oid,n.nspowner,'MEMBER')) OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname !~ '^pg_(temp|toast_temp)_[0-9]+$' AND has_schema_privilege($1::oid,n.oid,'CREATE')) OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname !~ '^pg_(temp|toast_temp)_[0-9]+$' AND c.relkind IN ('r','p','f','v','m','S') AND pg_has_role($1::oid,c.relowner,'MEMBER')) OR EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname !~ '^pg_(temp|toast_temp)_[0-9]+$' AND pg_has_role($1::oid,p.proowner,'MEMBER')) OR EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND p.prosecdef AND has_function_privilege($1::oid,p.oid,'EXECUTE')) OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','f') AND (has_table_privilege($1::oid,c.oid,'TRUNCATE') OR has_table_privilege($1::oid,c.oid,'TRIGGER')))`, roleOID, expected.DatabaseOID).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	return nil
}
