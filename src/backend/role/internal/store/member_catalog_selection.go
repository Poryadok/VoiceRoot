package store

import (
	"context"
	"fmt"

	"voice/backend/pkg/authoritysource"
)

func qualifiedRoleCatalog(version int64) (authoritysource.Catalog, error) {
	switch version {
	case 21:
		return roleAuthorityCatalog21, nil
	case 25:
		// Removal fences restrict future Bot mutations; they confer no mask,
		// source binding, or membership. Actual cleanup changes definitions and
		// typed contributions through the same guarded native21 Role clocks.
		// Consequently state2 facts/fold stay unchanged, with mandatory25 guards.
		return roleBotRemovalCatalog25, nil
	default:
		return authoritysource.Catalog{}, ErrScopeUnavailable
	}
}

func (s *RoleStore) checkQualifiedRoleCatalog(ctx context.Context) error {
	if s == nil || s.tx == nil {
		return ErrScopeUnavailable
	}
	var version int64
	if err := s.tx.QueryRow(ctx, `SELECT version FROM schema_migrations WHERE NOT dirty`).Scan(&version); err != nil {
		return fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	catalog, err := qualifiedRoleCatalog(version)
	if err == nil {
		err = authoritysource.CheckCatalog(ctx, s.tx, catalog)
	}
	if err != nil {
		return fmt.Errorf("%w: qualified Role catalog: %w", ErrScopeUnavailable, err)
	}
	return nil
}
