package grpcsvc

import (
	"context"

	"github.com/google/uuid"

	"voice/backend/search/internal/indexer"
)

// ChatReindexService backfills message search documents for a chat.
type ChatReindexService struct {
	Messages indexer.ChatMessageLister
	Store    indexer.MessageStore
}

func (c *ChatReindexService) ReindexChat(ctx context.Context, chatID uuid.UUID) error {
	if c == nil {
		return nil
	}
	return indexer.ReindexChat(ctx, chatID, c.Messages, c.Store)
}
