package sourcecustody

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ControlInventory identifies the independently selected upgrade identities.
type ControlInventory struct {
	Admin, Owner, Controller, Responder string
	ControlSystemID, ControlDatabase    string
	ControlDatabaseOID                  uint32
	LegacyLogins                        []string
}

// CheckControlInventory is a conservative, read-only snapshot of an isolated
// control cluster. It does not lock leadership, establish physical closure,
// fence sessions, qualify schema definitions, or authorize an upgrade. The
// caller must independently establish that the supplied inventory is complete.
// The result is immediately stale if any catalog, role or session changes.
func CheckControlInventory(ctx context.Context, conn *pgxpool.Conn, inventory ControlInventory) error {
	if ctx == nil || ctx.Err() != nil || conn == nil {
		return ErrUnavailable
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return ErrUnavailable
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if checkControlInventoryTx(ctx, tx, inventory) != nil || tx.Commit(ctx) != nil {
		return ErrUnavailable
	}
	return nil
}

// checkControlInventoryTx is a snapshot check within the caller's transaction;
// it does not itself acquire leader, role, table or source-fencing locks.
// A caller needing a fresh result must acquire protective locks first, use a
// fresh catalog snapshot and avoid an earlier cached pg_stat_activity snapshot
// (or clear the statistics snapshot before checking).
func checkControlInventoryTx(ctx context.Context, tx pgx.Tx, inventory ControlInventory) error {
	if ctx == nil || ctx.Err() != nil || tx == nil || len(inventory.LegacyLogins) == 0 || !systemID.MatchString(inventory.ControlSystemID) || !databaseName.MatchString(inventory.ControlDatabase) || inventory.ControlDatabaseOID == 0 {
		return ErrUnavailable
	}
	names := []string{inventory.Admin, inventory.Owner, inventory.Controller, inventory.Responder}
	names = append(names, inventory.LegacyLogins...)
	seen := make(map[string]bool, len(names))
	for _, name := range names {
		if !databaseName.MatchString(name) || seen[name] {
			return ErrUnavailable
		}
		seen[name] = true
	}
	var valid bool
	// Legacy roles may still own objects or be superusers at this preliminary
	// stage. They must be fenced by the separate future upgrade operation.
	err := tx.QueryRow(ctx, `SELECT
 current_user=$1 AND session_user=$1 AND current_database()=$8
 AND (SELECT system_identifier::text FROM pg_control_system())=$9
 AND (SELECT oid FROM pg_database WHERE datname=current_database())=$10::oid
 AND EXISTS(SELECT 1 FROM pg_roles WHERE rolname=$1 AND rolcanlogin AND rolsuper)
 AND (SELECT count(*) FROM pg_roles WHERE rolname=ANY($2::text[]))=cardinality($2::text[])
 AND NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolcanlogin AND NOT rolname=ANY($3::text[]))
 AND NOT EXISTS(SELECT 1 FROM pg_stat_activity WHERE backend_type='client backend' AND (usename IS NULL OR NOT usename=ANY($3::text[])))
 AND NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=ANY($4::text[]) AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls))
 AND NOT EXISTS(SELECT 1 FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.member JOIN pg_roles p ON p.oid=m.roleid WHERE r.rolname=ANY($2::text[]) OR p.rolname=ANY($2::text[]))
 AND NOT EXISTS(SELECT 1 FROM pg_prepared_xacts WHERE database=current_database())
 AND NOT EXISTS(SELECT 1 FROM pg_database d CROSS JOIN LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) a WHERE d.datname=current_database() AND a.grantee=0 AND a.privilege_type='CONNECT')
 AND (SELECT pg_get_userbyid(datdba)=$1 FROM pg_database WHERE datname=current_database())
 AND NOT EXISTS(SELECT 1 FROM pg_namespace n CROSS JOIN LATERAL aclexplode(COALESCE(n.nspacl,acldefault('n',n.nspowner))) a WHERE n.nspname='public' AND a.grantee=0 AND a.privilege_type='CREATE')
 AND (SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($5::text[]) AND c.relkind='r' AND pg_get_userbyid(c.relowner)=ANY($6::text[]))=cardinality($5::text[])
 AND NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) a LEFT JOIN pg_roles r ON r.oid=a.grantee WHERE n.nspname='public' AND c.relname=ANY($5::text[]) AND (a.grantee=0 OR NOT r.rolname=ANY($6::text[])))
 AND NOT EXISTS(SELECT 1 FROM pg_attribute col JOIN pg_class c ON c.oid=col.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL aclexplode(col.attacl) a LEFT JOIN pg_roles r ON r.oid=a.grantee WHERE n.nspname='public' AND c.relname=ANY($5::text[]) AND (a.grantee=0 OR NOT r.rolname=ANY($6::text[])))
 AND NOT EXISTS(SELECT 1 FROM pg_proc p WHERE p.prosecdef)
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON true JOIN pg_namespace n ON n.oid=p.pronamespace WHERE r.rolname=ANY($4::text[]) AND n.nspname='pg_catalog' AND has_function_privilege(r.oid,p.oid,'EXECUTE') AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON true JOIN pg_namespace n ON n.oid=p.pronamespace WHERE r.rolname=ANY($4::text[]) AND n.nspname='pg_catalog' AND p.proname=ANY($11::text[]) AND has_function_privilege(r.oid,p.oid,'EXECUTE'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_settings s ON true WHERE r.rolname=ANY($4::text[]) AND ((s.context IN ('superuser','superuser-backend') AND has_parameter_privilege(r.oid,s.name,'SET')) OR has_parameter_privilege(r.oid,s.name,'ALTER SYSTEM')))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_database d ON d.datname=current_database() WHERE r.rolname=ANY($4::text[]) AND has_database_privilege(r.oid,d.oid,'CREATE'))
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_class c ON c.relowner=r.oid WHERE r.rolname=ANY($4::text[]) AND r.rolname<>$7)
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON p.proowner=r.oid WHERE r.rolname=ANY($4::text[]) AND r.rolname<>$7)
 AND NOT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_namespace n ON true WHERE r.rolname=ANY($4::text[]) AND has_schema_privilege(r.oid,n.oid,'CREATE'))`,
		inventory.Admin, names, append([]string{inventory.Admin}, inventory.LegacyLogins...), []string{inventory.Owner, inventory.Controller, inventory.Responder},
		[]string{"user_custody_creation", "user_custody_physical_source", "user_custody_open_evidence", "user_custody_state"}, append([]string{inventory.Admin, inventory.Owner}, inventory.LegacyLogins...), inventory.Owner, inventory.ControlDatabase, inventory.ControlSystemID, inventory.ControlDatabaseOID,
		[]string{"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file", "pg_ls_logdir", "pg_ls_waldir", "pg_ls_archive_statusdir", "pg_ls_tmpdir", "pg_ls_replslotdir", "pg_ls_logicalmapdir", "pg_ls_logicalsnapdir", "lo_import", "lo_export", "pg_reload_conf", "pg_rotate_logfile", "pg_promote", "pg_create_restore_point", "pg_switch_wal", "pg_backup_start", "pg_backup_stop"}).Scan(&valid)
	if err != nil || !valid {
		return ErrUnavailable
	}
	return nil
}
