package store

import (
	"context"
	"github.com/google/uuid"
)

// AddBotMember rejects raw callers without installation journal or source-removal authority.
func (s *SpaceStore) AddBotMember(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s != nil && s.Pool != nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(*SpaceStore) error {
			return ErrMemberAuthorityUnavailable
		})
	}
	return ErrMemberAuthorityUnavailable
}
func (s *SpaceStore) RemoveBotMember(ctx context.Context, spaceID, profileID uuid.UUID) error {
	if s != nil && s.Pool != nil {
		return s.withOwnershipScope(ctx, []uuid.UUID{spaceID}, func(*SpaceStore) error {
			return ErrMemberAuthorityUnavailable
		})
	}
	return ErrMemberAuthorityUnavailable
}
