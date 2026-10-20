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

func terminalFunctionNames() []string {
	names := make([]string, len(roleBotGlobalTerminalCatalog26.Functions))
	for i, f := range roleBotGlobalTerminalCatalog26.Functions {
		names[i] = f.Name
	}
	return names
}
