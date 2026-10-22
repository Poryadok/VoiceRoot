package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/jackc/pgx/v5"
)

const accountBanStructureSQL = `SELECT jsonb_build_object(
 'columns',(SELECT jsonb_agg(jsonb_build_array(c.relname,to_jsonb(a)-'attrelid',format_type(a.atttypid,a.atttypmod),pg_get_expr(d.adbin,d.adrelid)) ORDER BY c.relname,a.attnum)
 FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
 WHERE a.attnum>0 AND NOT a.attisdropped AND n.nspname='public' AND c.relname=ANY($1)),
 'constraints',(SELECT jsonb_agg(jsonb_build_array(c.relname,k.conname,k.contype,k.condeferrable,k.condeferred,k.convalidated,to_jsonb(k)->>'conenforced',pg_get_constraintdef(k.oid)) ORDER BY c.relname,k.conname)
 FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)),
 'indexes',(SELECT jsonb_agg(jsonb_build_array(c.relname,i.indisunique,i.indisvalid,i.indisready,pg_get_indexdef(i.indexrelid)) ORDER BY c.relname,pg_get_indexdef(i.indexrelid))
 FROM pg_index i JOIN pg_class c ON c.oid=i.indrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)),
 'security',(SELECT jsonb_agg(jsonb_build_array(c.relname,
 c.relowner=(SELECT e.relowner FROM pg_class e JOIN pg_namespace en ON en.oid=e.relnamespace WHERE en.nspname='native_membership_guard' AND e.relname='epoch'),
 NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) a WHERE a.grantee<>c.relowner AND
 (a.grantee<>(SELECT oid FROM pg_roles WHERE rolname=session_user) OR a.is_grantable OR a.privilege_type NOT IN ('SELECT','INSERT','UPDATE','DELETE')))) ORDER BY c.relname)
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)),
 'triggers',(SELECT jsonb_agg(jsonb_build_array(c.relname,t.tgname,t.tgtype,t.tgenabled,t.tgdeferrable,t.tginitdeferred,encode(t.tgargs,'hex'),pn.nspname,p.proname,
 pg_get_triggerdef(t.oid,true),t.tgattr::text,pg_get_expr(t.tgqual,t.tgrelid),t.tgoldtable,t.tgnewtable,
 p.pronargs=0 AND p.prorettype='trigger'::regtype) ORDER BY c.relname,t.tgname)
 FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
 WHERE NOT t.tgisinternal AND n.nspname='public' AND c.relname=ANY($1)),
 'foreign_key_triggers',(SELECT jsonb_agg(jsonb_build_array(c.relname,k.conname,rn.nspname,r.relname,t.tgisinternal,t.tgtype,t.tgenabled,t.tgdeferrable,t.tginitdeferred,
 CASE WHEN t.tgname~'^RI_ConstraintTrigger_a_[0-9]+$' THEN 'RI_action' WHEN t.tgname~'^RI_ConstraintTrigger_c_[0-9]+$' THEN 'RI_check' ELSE t.tgname END,
 replace(pg_get_triggerdef(t.oid,true),quote_ident(t.tgname),'<generated-fk-trigger>'),t.tgattr::text,pg_get_expr(t.tgqual,t.tgrelid),encode(t.tgargs,'hex'),t.tgnargs,t.tgoldtable,t.tgnewtable,t.tgparentid=0,
 cn.nspname,cr.relname,CASE WHEN t.tgconstrindid=0 THEN NULL ELSE pg_get_indexdef(t.tgconstrindid) END,
 pn.nspname,p.proname,pg_get_function_identity_arguments(p.oid),p.prorettype::regtype::text)
 ORDER BY c.relname,k.conname,rn.nspname,r.relname,t.tgtype,pn.nspname,p.proname)
 FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_trigger t ON t.tgconstraint=k.oid
 JOIN pg_class r ON r.oid=t.tgrelid JOIN pg_namespace rn ON rn.oid=r.relnamespace
 JOIN pg_proc p ON p.oid=t.tgfoid JOIN pg_namespace pn ON pn.oid=p.pronamespace
 LEFT JOIN pg_class cr ON cr.oid=t.tgconstrrelid LEFT JOIN pg_namespace cn ON cn.oid=cr.relnamespace
 WHERE k.contype='f' AND n.nspname='public' AND c.relname=ANY($1))
 )::text`

var accountBanTables = []string{"space_account_ban_operations", "space_account_ban_captures", "space_account_ban_reconciliation", "space_account_qualifications", "space_account_ban_suppressions", "space_account_roster_projections"}

// Compiled only from a fresh source-bound migration fixture, never enrolled
// from a deployment's live catalog. Until qualified this cut fails closed.
const accountBanStructure16SHA = "d5a5fd0e74da956772061c5f84c1ab0393373cfaff0ea6e1e8a8a94cafce2036"
const accountBanStructure18SHA = "271034e23f93442d9126b0db8dbb5a57d7e781359af1de34318dfe960658c183"

func checkAccountBanCatalog(ctx context.Context, tx pgx.Tx) error {
	for _, table := range accountBanTables {
		if _, err := tx.Exec(ctx, "LOCK TABLE "+pgx.Identifier{"public", table}.Sanitize()+" IN ACCESS SHARE MODE"); err != nil {
			return ErrAccountBanUnavailable
		}
	}
	var version int
	if err := tx.QueryRow(ctx, `SELECT current_setting('server_version_num')::integer`).Scan(&version); err != nil {
		return ErrAccountBanUnavailable
	}
	expected, supported := map[int]string{16: accountBanStructure16SHA, 18: accountBanStructure18SHA}[version/10000]
	if !supported {
		return ErrAccountBanUnavailable
	}
	var structure string
	if err := tx.QueryRow(ctx, accountBanStructureSQL, accountBanTables).Scan(&structure); err != nil {
		return ErrAccountBanUnavailable
	}
	hash := sha256.Sum256([]byte(structure))
	if hex.EncodeToString(hash[:]) != expected {
		return ErrAccountBanUnavailable
	}
	var valid bool
	for name, pin := range map[string]string{"links": "8e7ffadc96b976d961e24036c72ad9c3f81a1815e5b238e08c6d744d3a16a3bb", "commit_guard": "3de8a3609bce7513a54fca32db7993643251168614fdb483e0ae1453c905a7c7"} {
		var body string
		err := tx.QueryRow(ctx, `SELECT p.prosrc,p.prosecdef AND NOT p.proleakproof AND NOT p.proretset AND NOT p.proisstrict AND p.provolatile='v' AND p.prorettype='trigger'::regtype
 AND p.prokind='f' AND p.proparallel='u' AND p.prosupport=0 AND p.pronargdefaults=0 AND p.proargdefaults IS NULL AND p.proargnames IS NULL AND p.procost=100 AND p.prorows=0
 AND p.proargtypes=''::oidvector AND p.proconfig=ARRAY['search_path=pg_catalog, public, space_account_ban_guard, pg_temp']::text[]
 AND l.lanname='plpgsql' AND p.proowner=n.nspowner AND p.proowner=(SELECT relowner FROM pg_class WHERE oid='public.spaces'::regclass)
 AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(p.proacl,acldefault('f',p.proowner))) a WHERE a.grantee<>p.proowner)
 AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(n.nspacl,acldefault('n',n.nspowner))) a WHERE a.grantee<>n.nspowner)
 AND (SELECT count(*) FROM pg_proc WHERE pronamespace=n.oid)=2
 AND NOT EXISTS(SELECT 1 FROM pg_class WHERE relnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_type WHERE typnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_operator WHERE oprnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_collation WHERE collnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_conversion WHERE connamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_opclass WHERE opcnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_opfamily WHERE opfnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_ts_config WHERE cfgnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_ts_dict WHERE dictnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_ts_parser WHERE prsnamespace=n.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_ts_template WHERE tmplnamespace=n.oid)
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang WHERE n.nspname='space_account_ban_guard' AND p.proname=$1`, name).Scan(&body, &valid)
		if err != nil || !valid {
			return ErrAccountBanUnavailable
		}
		hash = sha256.Sum256([]byte(body))
		if hex.EncodeToString(hash[:]) != pin {
			return ErrAccountBanUnavailable
		}
	}
	if err := tx.QueryRow(ctx, `SELECT count(*)=6 AND bool_and(c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
 AND NOT EXISTS(SELECT 1 FROM pg_attribute a WHERE a.attrelid=c.oid AND a.attnum>0 AND a.attisdropped)
 AND c.relowner=(SELECT e.relowner FROM pg_class e JOIN pg_namespace en ON en.oid=e.relnamespace WHERE en.nspname='native_membership_guard' AND e.relname='epoch')
 AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) a WHERE a.grantee<>c.relowner AND
 (a.grantee<>(SELECT oid FROM pg_roles WHERE rolname=session_user) OR a.is_grantable OR a.privilege_type NOT IN ('SELECT','INSERT','UPDATE','DELETE')))
 AND has_table_privilege(session_user,c.oid,'SELECT') AND has_table_privilege(session_user,c.oid,'INSERT')
 AND has_table_privilege(session_user,c.oid,'UPDATE') AND has_table_privilege(session_user,c.oid,'DELETE')
 AND NOT EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=c.oid) AND NOT EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=c.oid)
 AND NOT EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid))
 AND NOT EXISTS(SELECT 1 FROM pg_class x JOIN pg_namespace xn ON xn.oid=x.relnamespace
 WHERE xn.nspname='public' AND starts_with(x.relname,'space_account_')
 AND NOT (x.relname=ANY($1) OR (x.relkind IN ('i','I') AND EXISTS(SELECT 1 FROM pg_index i JOIN pg_class it ON it.oid=i.indrelid JOIN pg_namespace itn ON itn.oid=it.relnamespace WHERE i.indexrelid=x.oid AND itn.nspname='public' AND it.relname=ANY($1)))))
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY($1)`, accountBanTables).Scan(&valid); err != nil || !valid {
		return ErrAccountBanUnavailable
	}
	return nil
}
