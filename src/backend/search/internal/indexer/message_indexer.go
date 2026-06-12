package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

// MessageStore indexes message documents.
type MessageStore interface {
	Upsert(ctx context.Context, doc store.MessageDocument) error
	Delete(ctx context.Context, messageID uuid.UUID) error
}

// MessagingClient fetches message bodies for indexing.
type MessagingClient interface {
	GetMessageBody(ctx context.Context, chatID, messageID uuid.UUID) (string, time.Time, error)
}

// MessageIndexer handles message.events payloads for search projections.
type MessageIndexer struct {
	Store     MessageStore
	Messaging MessagingClient
}

// Handle processes a single message stream event.
func (idx *MessageIndexer) Handle(ctx context.Context, env *eventsv1.MessageStreamEvent) error {
	if idx == nil || env == nil {
		return nil
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.MessageStreamEvent_MessageSent:
		return idx.handleUpsert(ctx, p.MessageSent.GetChatId(), p.MessageSent.GetMessageId(), p.MessageSent.GetSenderProfileId())
	case *eventsv1.MessageStreamEvent_MessageEdited:
		return idx.handleUpsert(ctx, p.MessageEdited.GetChatId(), p.MessageEdited.GetMessageId(), "")
	case *eventsv1.MessageStreamEvent_MessageDeleted:
		return idx.handleDelete(ctx, p.MessageDeleted.GetMessageId())
	default:
		return nil
	}
}

func (idx *MessageIndexer) handleUpsert(ctx context.Context, chatRaw, msgRaw, senderRaw string) error {
	if idx.Store == nil || idx.Messaging == nil {
		return fmt.Errorf("message indexer not configured")
	}
	chatID, err := uuid.Parse(chatRaw)
	if err != nil {
		return fmt.Errorf("invalid chat_id: %w", err)
	}
	msgID, err := uuid.Parse(msgRaw)
	if err != nil {
		return fmt.Errorf("invalid message_id: %w", err)
	}
	body, createdAt, err := idx.Messaging.GetMessageBody(ctx, chatID, msgID)
	if err != nil {
		return err
	}
	var sender uuid.UUID
	if senderRaw != "" {
		sender, err = uuid.Parse(senderRaw)
		if err != nil {
			return fmt.Errorf("invalid sender_profile_id: %w", err)
		}
	}
	return idx.Store.Upsert(ctx, store.MessageDocument{
		MessageID:       msgID,
		ChatID:          chatID,
		SenderProfileID: sender,
		Body:            body,
		CreatedAt:       createdAt,
	})
}

func (idx *MessageIndexer) handleDelete(ctx context.Context, msgRaw string) error {
	if idx.Store == nil {
		return fmt.Errorf("message indexer not configured")
	}
	msgID, err := uuid.Parse(msgRaw)
	if err != nil {
		return fmt.Errorf("invalid message_id: %w", err)
	}
	return idx.Store.Delete(ctx, msgID)
}
