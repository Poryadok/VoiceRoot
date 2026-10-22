package store

import (
	"context"
	"github.com/google/uuid"
)

// AddBotMember rejects raw callers without installation journal or source-removal authority.
func (s *SpaceStore) AddBotMember(context.Context, uuid.UUID, uuid.UUID) error {
	return ErrMemberAuthorityUnavailable
}
func (s *SpaceStore) RemoveBotMember(context.Context, uuid.UUID, uuid.UUID) error {
	return ErrMemberAuthorityUnavailable
}
