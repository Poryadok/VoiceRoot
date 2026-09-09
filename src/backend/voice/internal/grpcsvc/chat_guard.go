package grpcsvc

import (
	"context"
	"errors"

	chatv1 "voice.app/voice/chat/v1"
)

// ErrNotChatMember is returned when a profile is not a member of the chat.
var ErrNotChatMember = errors.New("not a chat member")

// ErrNotDirectChat is returned when a linked chat is not a direct message.
var ErrNotDirectChat = errors.New("not a direct chat")

// DirectChatValidator validates that a linked chat is an authoritative DM.
type DirectChatValidator interface {
	EnsureDirectChat(ctx context.Context, chatID string) error
}

// ChatMembership validates that a profile belongs to a chat (DM or group).
type ChatMembership interface {
	EnsureMember(ctx context.Context, chatID, profileID string) error
}

type mapChatMembers struct {
	members map[string]map[string]bool
	types   map[string]chatv1.ChatType
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

func (m *mapChatMembers) EnsureDirectChat(_ context.Context, chatID string) error {
	if m == nil || m.types == nil || m.types[chatID] == chatv1.ChatType_CHAT_TYPE_DM {
		return nil
	}
	return ErrNotDirectChat
}

// ErrNotSpaceMember is returned when a profile is not a member of the space.
var ErrNotSpaceMember = errors.New("not a space member")

// SpaceMembership validates that a profile belongs to a space.
type SpaceMembership interface {
	EnsureMember(ctx context.Context, spaceID, profileID string) error
}

type mapSpaceMembers struct {
	members map[string]map[string]bool
}

func (m *mapSpaceMembers) EnsureMember(_ context.Context, spaceID, profileID string) error {
	if m == nil {
		return nil
	}
	space, ok := m.members[spaceID]
	if !ok || !space[profileID] {
		return ErrNotSpaceMember
	}
	return nil
}
