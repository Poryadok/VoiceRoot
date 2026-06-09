package grpcsvc

import (
	"context"
	"errors"
)

// ErrNotChatMember is returned when a profile is not a member of the chat.
var ErrNotChatMember = errors.New("not a chat member")

// ChatMembership validates that a profile belongs to a chat (DM or group).
type ChatMembership interface {
	EnsureMember(ctx context.Context, chatID, profileID string) error
}

type mapChatMembers struct {
	members map[string]map[string]bool
}

func (m *mapChatMembers) EnsureMember(_ context.Context, chatID, profileID string) error {
	if m == nil {
		return nil
	}
	chat, ok := m.members[chatID]
	if !ok || !chat[profileID] {
		return ErrNotChatMember
	}
	return nil
}
