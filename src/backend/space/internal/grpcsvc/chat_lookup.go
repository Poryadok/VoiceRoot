package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	chatv1 "voice.app/voice/chat/v1"
)

// ChatInfo holds display metadata for a linked chat node.
type ChatInfo struct {
	Name     string
	ChatType chatv1.ChatType
}

// ChatLookup resolves chat display names and types for space tree enrichment.
type ChatLookup interface {
	GetChatNames(ctx context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]ChatInfo, error)
}

type mapChatLookup struct {
	chats map[uuid.UUID]ChatInfo
}

func (m *mapChatLookup) GetChatNames(_ context.Context, chatIDs []uuid.UUID) (map[uuid.UUID]ChatInfo, error) {
	out := make(map[uuid.UUID]ChatInfo, len(chatIDs))
	for _, id := range chatIDs {
		if info, ok := m.chats[id]; ok {
			out[id] = info
		}
	}
	return out, nil
}
