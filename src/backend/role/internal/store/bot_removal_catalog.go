package store

import (
	"context"
	"fmt"

	"voice/backend/pkg/authoritysource"
)

// The25 extension is shared by cleanup and the unchanged state2 mask fold.
// Its presence does not authorize Bot actor mutations or confer membership.
// Cleanup removes typed preparations/contributions before definitions, so all
// native21 clocks and immutable witnesses remain mandatory dependencies.
var roleBotRemovalCatalog25 = func() authoritysource.Catalog {
	c := roleAuthorityCatalog21
	c.Version = 25
	c.Tables = append(append([]string(nil), c.Tables...), "role_bot_actor_bindings", "role_bot_global_removals", "role_bot_installation_removals", "role_bot_removal_operations")
	c.Triggers = append(append([]authoritysource.CatalogTrigger(nil), c.Triggers...),
		authoritysource.CatalogTrigger{Table: "role_bot_actor_bindings", Name: "role_bot_actor_bindings_guard", Function: "role_bot_removal_immutable", Type: 58},
		authoritysource.CatalogTrigger{Table: "role_bot_global_removals", Name: "role_bot_global_removals_guard", Function: "role_bot_removal_immutable", Type: 58},
		authoritysource.CatalogTrigger{Table: "role_bot_installation_removals", Name: "role_bot_installation_removals_guard", Function: "role_bot_removal_immutable", Type: 58},
		authoritysource.CatalogTrigger{Table: "role_bot_removal_operations", Name: "role_bot_removal_operations_change", Function: "role_bot_removal_immutable", Type: 27},
		authoritysource.CatalogTrigger{Table: "role_bot_removal_operations", Name: "role_bot_removal_operations_truncate", Function: "role_bot_removal_immutable", Type: 34},
	)
	c.Functions = append(append([]authoritysource.CatalogFunction(nil), c.Functions...), authoritysource.CatalogFunction{Name: "role_bot_removal_immutable", Arguments: 0, ReturnType: "trigger", BodySHA256: "4ca077ff39e0a267729a13374bf69e174fc9b8a7b39e9d68df726020e304f34e"})
	return c
}()

func (s *RoleStore) checkBotRemovalCatalog(ctx context.Context) error {
	_, err := s.botGlobalTerminalCatalogPresent(ctx)
	return err
}

// botGlobalTerminalCatalogPresent returns a mode only after the complete exact
// catalog proof in the caller's transaction. Neither absence nor an unlocked
// version hint permits a legacy fallback. The proof retains schema locks.
func (s *RoleStore) botGlobalTerminalCatalogPresent(ctx context.Context) (bool, error) {
	if s == nil || s.tx == nil {
		return false, ErrScopeUnavailable
	}
	var version int64
	if err := s.tx.QueryRow(ctx, "SELECT version FROM public.schema_migrations WHERE NOT dirty").Scan(&version); err != nil {
		return false, fmt.Errorf("%w: Bot cleanup catalog", ErrScopeUnavailable)
	}
	if version == 26 {
		if err := s.checkBotGlobalTerminalCatalog(ctx); err != nil {
			return false, err
		}
		return true, nil
	}
	if version != 25 {
		return false, ErrScopeUnavailable
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleBotRemovalCatalog25); err != nil {
		return false, fmt.Errorf("%w: Bot cleanup catalog: %w", ErrScopeUnavailable, err)
	}
	return false, nil
}
