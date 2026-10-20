package store

import (
	"context"
	"fmt"

	"voice/backend/pkg/authoritysource"
)

var roleBotGlobalTerminalCatalog26 = func() authoritysource.Catalog {
	c := roleBotRemovalCatalog25
	c.Version = 26
	c.Tables = append(append([]string(nil), c.Tables...), "role_bot_global_terminals")
	c.Triggers = append(append([]authoritysource.CatalogTrigger(nil), c.Triggers...), authoritysource.CatalogTrigger{Table: "role_bot_global_terminals", Name: "role_bot_global_terminals_guard", Function: "role_bot_removal_immutable", Type: 58})
	return c
}()

// checkBotGlobalTerminalCatalog qualifies the root-only writer's actual
// transaction and serving LOGIN. Structural catalog checks alone prove no ACL.
func (s *RoleStore) checkBotGlobalTerminalCatalog(ctx context.Context) error {
	if s == nil || s.tx == nil {
		return ErrScopeUnavailable
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleBotGlobalTerminalCatalog26); err != nil {
		return fmt.Errorf("%w: Role terminal catalog: %w", ErrScopeUnavailable, err)
	}
	if err := s.checkBotTerminalShape(ctx); err != nil { return fmt.Errorf("%w: Role terminal relation shape: %w",ErrScopeUnavailable,err) }
	var valid bool
	err := s.tx.QueryRow(ctx, `SELECT current_setting('transaction_isolation')='read committed'
 AND current_user=session_user AND NOT r.rolsuper AND NOT r.rolbypassrls
 AND NOT r.rolcreaterole AND NOT r.rolcreatedb AND NOT r.rolreplication
 AND has_schema_privilege(current_user,'public','USAGE') AND NOT has_schema_privilege(current_user,'public','CREATE')
 AND has_table_privilege(current_user,c.oid,'SELECT') AND has_table_privilege(current_user,c.oid,'INSERT')
 AND NOT has_table_privilege(current_user,c.oid,'UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER')
 AND NOT EXISTS(SELECT 1 FROM aclexplode(COALESCE(c.relacl,acldefault('r',c.relowner))) a WHERE a.grantee=0)
 AND NOT EXISTS(SELECT 1 FROM pg_class x JOIN pg_namespace n ON n.oid=x.relnamespace WHERE n.nspname='public' AND x.relname=ANY($1) AND pg_has_role(current_user,x.relowner,'MEMBER'))
 AND NOT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proname=ANY($2) AND pg_has_role(current_user,p.proowner,'MEMBER'))
 FROM pg_roles r CROSS JOIN pg_class c WHERE r.rolname=current_user AND c.oid='public.role_bot_global_terminals'::regclass`, roleBotGlobalTerminalCatalog26.Tables, terminalFunctionNames()).Scan(&valid)
	if err != nil || !valid {
		return fmt.Errorf("%w: Role terminal serving privileges unavailable", ErrScopeUnavailable)
	}
	return nil
}

func (s *RoleStore) checkBotTerminalShape(ctx context.Context) error {
	var valid bool
	err:=s.tx.QueryRow(ctx,`WITH expected(number,name,kind,default_expr) AS (VALUES
 (1,'bot_id','uuid',''),(2,'decision_id','uuid',''),(3,'receipt_id','uuid',''),(4,'registered_at','timestamp with time zone','clock_timestamp()'))
 SELECT (SELECT count(*) FROM pg_attribute WHERE attrelid='public.role_bot_global_terminals'::regclass AND attnum>0 AND NOT attisdropped)=4
 AND (SELECT count(*) FROM pg_trigger WHERE tgrelid='public.role_bot_global_terminals'::regclass AND NOT tgisinternal)=1
 AND NOT EXISTS(SELECT 1 FROM expected e LEFT JOIN pg_attribute a ON a.attrelid='public.role_bot_global_terminals'::regclass AND a.attnum=e.number AND NOT a.attisdropped
 LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum
 WHERE a.attname IS DISTINCT FROM e.name OR a.atttypid IS DISTINCT FROM to_regtype(e.kind) OR NOT a.attnotnull OR a.attidentity<>'' OR a.attgenerated<>'' OR a.attcollation<>0 OR COALESCE(pg_get_expr(d.adbin,d.adrelid),'')<>e.default_expr)`).Scan(&valid)
	if err != nil || !valid { return fmt.Errorf("minimal columns/default/trigger unavailable") }
	expected:=map[string]struct{kind,key,expression string}{
		"role_bot_global_terminals_pkey":{"p","{1}",""},
		"role_bot_global_terminals_decision_id_key":{"u","{2}",""},
		"role_bot_global_terminals_receipt_id_key":{"u","{3}",""},
		"role_bot_global_terminals_bot_id_check":{"c","{1}","(bot_id <> '00000000-0000-0000-0000-000000000000'::uuid)"},
		"role_bot_global_terminals_decision_id_check":{"c","{2}","(decision_id <> '00000000-0000-0000-0000-000000000000'::uuid)"},
		"role_bot_global_terminals_receipt_id_check":{"c","{3}","(receipt_id <> '00000000-0000-0000-0000-000000000000'::uuid)"},
		"role_bot_global_terminals_registered_at_check":{"c","{4}","isfinite(registered_at)"},
	}
	rows,err:=s.tx.Query(ctx,`SELECT c.conname,c.contype::text,c.conkey::text,COALESCE(pg_get_expr(c.conbin,c.conrelid),''),c.convalidated AND NOT c.condeferrable AND c.conislocal
 AND (c.contype='c' OR EXISTS(SELECT 1 FROM pg_index i JOIN pg_class x ON x.oid=i.indexrelid JOIN pg_am m ON m.oid=x.relam JOIN pg_opclass o ON o.oid=i.indclass[0] JOIN pg_namespace n ON n.oid=o.opcnamespace
 WHERE i.indexrelid=c.conindid AND i.indisunique AND i.indisvalid AND i.indisready AND i.indnatts=1 AND i.indpred IS NULL AND i.indexprs IS NULL AND m.amname='btree' AND n.nspname='pg_catalog' AND o.opcname='uuid_ops'))
 FROM pg_constraint c WHERE c.conrelid='public.role_bot_global_terminals'::regclass`)
	if err != nil { return err };defer rows.Close()
	count:=0
	for rows.Next(){var name,kind,key,expression string;var qualified bool;if err:=rows.Scan(&name,&kind,&key,&expression,&qualified);err!=nil{return err};want,ok:=expected[name];if !ok||!qualified||want.kind!=kind||want.key!=key||want.expression!=expression{return fmt.Errorf("minimal constraint unavailable")};count++}
	if err:=rows.Err();err!=nil{return err};if count!=len(expected){return fmt.Errorf("minimal constraint set unavailable")}
	return nil
}

func terminalFunctionNames() []string {
	names := make([]string, len(roleBotGlobalTerminalCatalog26.Functions))
	for i, f := range roleBotGlobalTerminalCatalog26.Functions {
		names[i] = f.Name
	}
	return names
}
