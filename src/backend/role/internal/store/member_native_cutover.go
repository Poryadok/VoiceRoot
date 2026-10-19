package store

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"time"
	"voice/backend/pkg/authoritysource"
	"voice/backend/role/internal/memberauthority"
)

// NativeMembershipRequired is an owner-schema decision, never a request flag.
// Historical catalogs retain their separate behavior; future/dirty schemas do
// not fall back to historical authorization.
func (s *RoleStore) NativeMembershipRequired(ctx context.Context) (bool, error) {
	if s == nil || (s.tx == nil && s.Pool == nil) {
		return false, ErrScopeUnavailable
	}
	var db scopeExecutor = s.Pool
	if s.tx != nil {
		db = s.db()
	}
	var version int64
	var dirty bool
	if err := db.QueryRow(ctx, `SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		return false, fmt.Errorf("%w: native schema: %w", ErrScopeUnavailable, err)
	}
	if dirty || version < 1 || version > 21 {
		return false, ErrScopeUnavailable
	}
	return version == 21, nil
}

// QualifiedRevision is local-only and must run inside the caller's Space fence.
// It binds the later mutation CAS to both independent Role-owned clocks.
func (s *RoleStore) QualifiedRevision(ctx context.Context, space uuid.UUID) (uint64, error) {
	if s == nil || s.tx == nil {
		return 0, ErrScopeUnavailable
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleAuthorityCatalog21); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	role, sdk, err := roleSourceRevisions(ctx, s.tx, space.String())
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	return role + sdk, nil
}

// QualifiedMemberPresent distinguishes current membership from possessing a
// role. Native episodes need not carry an implicit Member/default grant.
func (s *RoleStore) QualifiedMemberPresent(ctx context.Context, space, profile uuid.UUID) (bool, error) {
	if s == nil || s.tx == nil {
		return false, ErrScopeUnavailable
	}
	if _, fenced := s.scopeSpaces[space]; !fenced {
		return false, ErrScopeUnavailable
	}
	observation, ok := memberauthority.FromContext(ctx, space.String(), profile.String())
	if !ok || !observation.Available(time.Now()) {
		return false, ErrScopeUnavailable
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleAuthorityCatalog21); err != nil {
		return false, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	state, err := readMemberRoleState(ctx, s.db(), space.String())
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	member, err := authoritysource.QualifyRoleMember(state, observation, profile.String(), "", "", time.Now())
	if err != nil {
		return false, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	if !observation.Available(time.Now()) {
		return false, ErrScopeUnavailable
	}
	return member.EligibleMember, nil
}

// GetRawMemberRoleInventory is explicitly non-authoritative migration/cleanup
// inventory. Authorization and exported membership readers use GetMemberRoles.
func (s *RoleStore) GetRawMemberRoleInventory(ctx context.Context, space, profile uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, space, func(scoped *RoleStore) ([]RoleRow, error) { return scoped.rawMemberRoleInventory(ctx, space, profile) })
}
