package store

import (
	"context"
	"github.com/google/uuid"
)

// Raw callers have no installation journal or source-removal authority.
func (s *SpaceStore) AddBotMember(context.Context, uuid.UUID, uuid.UUID) error {
	return ErrMemberAuthorityUnavailable
}
func (s *SpaceStore) RemoveBotMember(context.Context, uuid.UUID, uuid.UUID) error {
	return ErrMemberAuthorityUnavailable
}
