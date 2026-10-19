package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"voice/backend/pkg/authoritysource"
	"voice/backend/role/internal/memberauthority"
)

// GetQualifiedMemberRoles is the native admission/Space-chat reader. It never
// calls remote services under a Role lock and never infers legacy provenance.
// The legacy reader cutover is a separate coherent activation step requiring
// all generic assignment/baseline writers and downstream source consumers.
func (s *RoleStore) GetQualifiedMemberRoles(ctx context.Context, space, profile uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, space, func(scoped *RoleStore) ([]RoleRow, error) {
		return scoped.qualifiedMemberRoles(ctx, space, profile, nil)
	})
}

func (s *RoleStore) GetQualifiedMemberRolesForChat(ctx context.Context, space, profile, chat uuid.UUID) ([]RoleRow, error) {
	return spaceValue(ctx, s, space, func(scoped *RoleStore) ([]RoleRow, error) {
		return scoped.qualifiedMemberRoles(ctx, space, profile, &chat)
	})
}

func (s *RoleStore) qualifiedMemberRoles(ctx context.Context, space, profile uuid.UUID, chat *uuid.UUID) ([]RoleRow, error) {
	var observation memberauthority.Observation
	var ok bool
	if chat == nil {
		observation, ok = memberauthority.FromContext(ctx, space.String(), profile.String())
	} else {
		observation, ok = memberauthority.FromChatContext(ctx, space.String(), profile.String(), chat.String())
		ok = ok && observation.BoundToChat(chat.String(), time.Now())
	}
	if !ok {
		return nil, fmt.Errorf("%w: current membership observation missing", ErrScopeUnavailable)
	}
	if !observation.Available(time.Now()) {
		return nil, fmt.Errorf("%w: membership observation inactive or expired", ErrScopeUnavailable)
	}
	if err := authoritysource.CheckCatalog(ctx, s.tx, roleAuthorityCatalog21); err != nil {
		return nil, fmt.Errorf("%w: native Role catalog: %w", ErrScopeUnavailable, err)
	}
	state, err := readMemberRoleState(ctx, s.db(), space.String())
	if err != nil {
		return nil, fmt.Errorf("%w: Role facts: %w", ErrScopeUnavailable, err)
	}
	chatID := ""
	if chat != nil {
		chatID = chat.String()
	}
	qualified, err := authoritysource.QualifyRoleMember(state, observation, profile.String(), chatID, "", time.Now())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrScopeUnavailable, err)
	}
	var out []RoleRow
	for _, role := range qualified.Roles {
		row, err := s.GetRoleByID(ctx, uuid.MustParse(role.ID))
		if err != nil {
			return nil, err
		}
		if row == nil {
			return nil, ErrScopeUnavailable
		}
		out = append(out, *row)
	}
	if !observation.Available(time.Now()) {
		return nil, ErrScopeUnavailable
	}
	return out, nil
}
func (s *RoleStore) GetQualifiedEffectiveMask(ctx context.Context, space, profile uuid.UUID, chat, room *uuid.UUID) (uint64, error) {
	return spaceValue(ctx, s, space, func(scoped *RoleStore) (uint64, error) {
		roles, err := scoped.qualifiedMemberRoles(ctx, space, profile, chat)
		if err != nil {
			return 0, err
		}
		return scoped.effectiveMaskForRoles(ctx, space, roles, chat, room, false)
	})
}
