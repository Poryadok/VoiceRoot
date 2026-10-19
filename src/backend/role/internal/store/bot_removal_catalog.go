package store

import (
	"context"
	"fmt"

	"voice/backend/pkg/authoritysource"
)

// This catalog qualifies only the dedicated cleanup transaction. It does not
// advertise native member/source2 serving at25 or authorize Bot actor mutations.
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
	if s == nil || s.tx == nil {
		return ErrScopeUnavailable
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleBotRemovalCatalog25); err != nil {
		return fmt.Errorf("%w: Bot cleanup catalog: %w", ErrScopeUnavailable, err)
	}
	return nil
}
