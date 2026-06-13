package indexer

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"voice/backend/search/internal/deps"
	storepkg "voice/backend/search/internal/store"
)

// ChatMessageLister pages messages for reindex.
type ChatMessageLister interface {
	ListChatMessages(ctx context.Context, chatID uuid.UUID, cursor string, pageSize int32) ([]deps.MessageRow, string, error)
}

// ReindexChat backfills message_search_documents from Messaging GetMessages.
func ReindexChat(ctx context.Context, chatID uuid.UUID, messages ChatMessageLister, msgStore MessageStore) error {
	if messages == nil || msgStore == nil {
		return fmt.Errorf("reindex chat: dependencies not configured")
	}
	cursor := ""
	const pageSize int32 = 100
	for {
		rows, next, err := messages.ListChatMessages(ctx, chatID, cursor, pageSize)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := msgStore.Upsert(ctx, storepkg.MessageDocument{
				MessageID:       row.ID,
				ChatID:          chatID,
				SenderProfileID: row.SenderProfileID,
				Body:            row.Body,
				CreatedAt:       row.CreatedAt,
			}); err != nil {
				return err
			}
		}
		if next == "" {
			return nil
		}
		cursor = next
	}
}
