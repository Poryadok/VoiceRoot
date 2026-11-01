package sourcecustody

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// stageControlCatalogFenceTx is a NON-AUTHORITATIVE, unwired prototype. It cannot
// establish physical source closure, rule out authenticating client backends,
// qualify the complete preexisting schema, revalidate after restart, or
// authorize activation. It must not be called by a production path until all
// those missing guarantees are implemented.
// The caller owns the transaction and must roll it back on any error.
func stageControlCatalogFenceTx(ctx context.Context, tx pgx.Tx, inventory ControlInventory) error {
	if ctx == nil || ctx.Err() != nil || tx == nil {
		return ErrUnavailable
	}
	if _, err := tx.Exec(ctx, `SET LOCAL search_path=pg_catalog`); err != nil {
		return ErrUnavailable
	}
	var held bool
	if tx.QueryRow(ctx, `SELECT current_setting('transaction_isolation')='read committed' AND EXISTS(SELECT 1 FROM pg_locks WHERE pid=pg_backend_pid() AND locktype='advisory' AND mode='ExclusiveLock' AND granted AND objsubid=1 AND classid=($1::bigint >> 32)::oid AND objid=($1::bigint & 4294967295)::oid)`, leaderKey).Scan(&held) != nil || !held {
		return ErrUnavailable
	}
	if _, err := tx.Exec(ctx, `LOCK TABLE pg_catalog.pg_authid,pg_catalog.pg_auth_members,pg_catalog.pg_event_trigger IN ACCESS EXCLUSIVE MODE`); err != nil {
		return ErrUnavailable
	}
	var unsafe bool
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_prepared_xacts) OR EXISTS(SELECT 1 FROM pg_event_trigger)`).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if _, err := tx.Exec(ctx, `LOCK TABLE public.user_custody_creation,public.user_custody_state,public.user_custody_physical_source,public.user_custody_open_evidence IN ACCESS EXCLUSIVE MODE; SELECT pg_stat_clear_snapshot()`); err != nil || checkControlInventoryTx(ctx, tx, inventory) != nil {
		return ErrUnavailable
	}
	// A previously authenticated superuser can retain session authority after
	// role demotion on supported PG versions. NOLOGIN is not a session fence.
	// Even this negative observation cannot rule out in-flight authentication.
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname=ANY($1::text[]) AND rolsuper) OR EXISTS(SELECT 1 FROM pg_stat_activity WHERE backend_type='client backend' AND usename=ANY($1::text[]))`, inventory.LegacyLogins).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1::text[]) AND (c.relrowsecurity OR c.relforcerowsecurity OR c.relhassubclass OR EXISTS(SELECT 1 FROM pg_trigger t WHERE t.tgrelid=c.oid AND NOT t.tgisinternal) OR EXISTS(SELECT 1 FROM pg_rewrite r WHERE r.ev_class=c.oid))) AND EXISTS(SELECT 1 FROM public.user_custody_creation c JOIN public.user_custody_state s ON s.singleton WHERE c.singleton AND c.phase IN ('INITIALIZED_CLOSED','EXISTING_INITIALIZED_CLOSED','RECOVERED_CLOSED') AND s.state='CLOSED' AND c.source_id=s.source_id AND c.epoch=s.epoch)`, controlFenceTables).Scan(&held) != nil || !held {
		return ErrUnavailable
	}
	if _, err := tx.Exec(ctx, `ALTER TABLE public.user_custody_creation ADD CONSTRAINT user_custody_supervision_fence_phase CHECK (phase NOT IN ('PREPARING_OPEN','ACTIVE')); ALTER TABLE public.user_custody_state ADD CONSTRAINT user_custody_supervision_fence_state CHECK (state='CLOSED')`); err != nil {
		return ErrUnavailable
	}
	for _, role := range inventory.LegacyLogins {
		identifier := pgx.Identifier{role}.Sanitize()
		if _, err := tx.Exec(ctx, `ALTER ROLE `+identifier+` NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`); err != nil {
			return ErrUnavailable
		}
		if _, err := tx.Exec(ctx, `REVOKE ALL ON public.user_custody_creation,public.user_custody_state,public.user_custody_physical_source,public.user_custody_open_evidence FROM `+identifier); err != nil {
			return ErrUnavailable
		}
		if _, err := tx.Exec(ctx, `REVOKE ALL ON DATABASE `+pgx.Identifier{inventory.ControlDatabase}.Sanitize()+` FROM `+identifier); err != nil {
			return ErrUnavailable
		}
	}
	for _, name := range controlFenceTables {
		if _, err := tx.Exec(ctx, `ALTER TABLE `+pgx.Identifier{"public", name}.Sanitize()+` OWNER TO `+pgx.Identifier{inventory.Owner}.Sanitize()); err != nil {
			return ErrUnavailable
		}
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_shdepend d ON d.refclassid='pg_authid'::regclass AND d.refobjid=r.oid WHERE r.rolname=ANY($1::text[])) OR EXISTS(SELECT 1 FROM pg_roles r JOIN pg_settings s ON true WHERE r.rolname=ANY($1::text[]) AND ((s.context IN ('superuser','superuser-backend') AND has_parameter_privilege(r.oid,s.name,'SET')) OR has_parameter_privilege(r.oid,s.name,'ALTER SYSTEM'))) OR EXISTS(SELECT 1 FROM pg_roles r JOIN pg_namespace n ON true WHERE r.rolname=ANY($1::text[]) AND has_schema_privilege(r.oid,n.oid,'CREATE')) OR EXISTS(SELECT 1 FROM pg_roles r JOIN pg_database d ON true WHERE r.rolname=ANY($1::text[]) AND has_database_privilege(r.oid,d.oid,'CREATE'))`, inventory.LegacyLogins).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	if tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_roles r JOIN pg_proc p ON true JOIN pg_namespace n ON n.oid=p.pronamespace WHERE r.rolname=ANY($1::text[]) AND n.nspname='pg_catalog' AND has_function_privilege(r.oid,p.oid,'EXECUTE') AND (p.proname=ANY($2::text[]) OR NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE')))`, inventory.LegacyLogins, []string{"pg_read_file", "pg_read_binary_file", "pg_ls_dir", "pg_stat_file", "pg_ls_logdir", "pg_ls_waldir", "pg_ls_archive_statusdir", "pg_ls_tmpdir", "pg_ls_replslotdir", "pg_ls_logicalmapdir", "pg_ls_logicalsnapdir", "lo_import", "lo_export", "pg_reload_conf", "pg_rotate_logfile", "pg_promote", "pg_create_restore_point", "pg_switch_wal", "pg_backup_start", "pg_backup_stop"}).Scan(&unsafe) != nil || unsafe {
		return ErrUnavailable
	}
	return nil
}

var controlFenceTables = []string{"user_custody_creation", "user_custody_state", "user_custody_physical_source", "user_custody_open_evidence"}
