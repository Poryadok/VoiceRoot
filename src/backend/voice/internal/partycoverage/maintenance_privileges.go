package partycoverage

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// maintenanceSQLScope is derived from the protected issuer's credential and
// source manifests, never from an owner-command argument or serving config.
// It is intentionally private while that concrete composition remains absent.
type maintenanceSQLScope struct {
	RoleOID, SchemaOID uint32
	RoleName           string
	WritableRelations  []uint32
}

// checkMaintenancePrivileges checks only the actual SQL session and grant
// boundary. It does not authenticate scope, prove a closed process composition,
// observe external retirement, or authorize any maintenance command by itself.
// A real authority must establish those independently before using this helper.
func checkMaintenancePrivileges(ctx context.Context, tx pgx.Tx, scope maintenanceSQLScope) error {
	if tx == nil || scope.RoleOID == 0 || scope.SchemaOID == 0 || scope.RoleName == "" || len(scope.WritableRelations) == 0 {
		return ErrUnavailable
	}
	var unsafe bool
	err := tx.QueryRow(ctx, `SELECT
 NOT EXISTS(SELECT 1 FROM pg_roles a JOIN pg_stat_activity b ON b.pid=pg_backend_pid()
 WHERE a.oid=$1::oid AND a.rolname=$2 AND a.rolcanlogin AND b.usesysid=a.oid
 AND current_user=a.rolname AND session_user=a.rolname)
 OR has_database_privilege(current_user,current_database(),'CREATE')
 OR EXISTS(SELECT 1 FROM pg_roles a WHERE pg_has_role($1::oid,a.oid,'MEMBER') AND
 (a.rolsuper OR a.rolcreaterole OR a.rolcreatedb OR a.rolreplication OR a.rolbypassrls
 OR a.rolname IN ('pg_write_server_files','pg_read_server_files','pg_execute_server_program')
 OR EXISTS(SELECT 1 FROM pg_proc p WHERE p.prosecdef AND has_function_privilege(a.oid,p.oid,'EXECUTE'))
 OR EXISTS(SELECT 1 FROM pg_namespace n WHERE n.nspname !~ '^pg_' AND n.nspname<>'information_schema' AND has_schema_privilege(a.oid,n.oid,'CREATE'))
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE n.nspname !~ '^pg_' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','f') AND
 (pg_has_role(a.oid,c.relowner,'MEMBER')
 OR has_table_privilege(a.oid,c.oid,'TRUNCATE,REFERENCES,TRIGGER')
 OR has_any_column_privilege(a.oid,c.oid,'REFERENCES')
 OR (NOT c.oid=ANY($4::oid[]) AND
 (has_table_privilege(a.oid,c.oid,'INSERT,UPDATE,DELETE') OR has_any_column_privilege(a.oid,c.oid,'INSERT,UPDATE')))))))
 OR EXISTS(SELECT 1 FROM unnest($4::oid[]) admitted(oid) LEFT JOIN pg_class c ON c.oid=admitted.oid
 WHERE c.oid IS NULL OR c.relnamespace<>$3::oid OR c.relkind NOT IN ('r','p')
 OR c.relname IN ('voice_party_coverage_head','voice_party_coverage_receipts','voice_party_coverage_operations','voice_party_coverage_steps',
 'voice_party_resource_components','voice_party_resource_retirements','voice_party_resource_component_links'))`, scope.RoleOID, scope.RoleName, scope.SchemaOID, scope.WritableRelations).Scan(&unsafe)
	if err != nil || unsafe {
		return ErrUnavailable
	}
	return nil
}
