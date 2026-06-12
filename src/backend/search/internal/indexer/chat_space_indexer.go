package indexer

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	eventsv1 "voice.app/voice/events/v1"

	"voice/backend/search/internal/store"
)

// ChatProjectionStore indexes chat titles for global search.
type ChatProjectionStore interface {
	UpsertChat(ctx context.Context, chatID uuid.UUID, title string) error
}

// SpaceProjectionStore indexes public space catalog rows.
type SpaceProjectionStore interface {
	UpsertSpace(ctx context.Context, doc store.SpaceDocument) error
	DeleteSpace(ctx context.Context, spaceID uuid.UUID) error
}

// ChatHydrator loads chat titles for indexing.
type ChatHydrator interface {
	LoadChatTitle(ctx context.Context, chatID uuid.UUID) (string, error)
}

// SpaceHydrator loads space catalog fields for indexing.
type SpaceHydrator interface {
	LoadSpace(ctx context.Context, spaceID uuid.UUID) (name, description, visibility string, memberCount int, err error)
}

// ChatSpaceIndexer handles chat.events payloads for search projections.
type ChatSpaceIndexer struct {
	Chats   ChatProjectionStore
	Spaces  SpaceProjectionStore
	ChatAPI ChatHydrator
	SpaceAPI SpaceHydrator
}

// Handle processes chat and space events.
func (idx *ChatSpaceIndexer) Handle(ctx context.Context, env *eventsv1.ChatStreamEvent) error {
	if idx == nil || env == nil {
		return nil
	}
	switch p := env.GetPayload().(type) {
	case *eventsv1.ChatStreamEvent_ChatCreated:
		return idx.handleChatCreated(ctx, p.ChatCreated.GetChatId())
	case *eventsv1.ChatStreamEvent_SpaceCreated:
		return idx.handleSpaceCreated(ctx, p.SpaceCreated.GetSpaceId())
	default:
		return nil
	}
}

func (idx *ChatSpaceIndexer) handleChatCreated(ctx context.Context, chatRaw string) error {
	if idx.Chats == nil || idx.ChatAPI == nil {
		return fmt.Errorf("chat indexer not configured")
	}
	chatID, err := uuid.Parse(chatRaw)
	if err != nil {
		return fmt.Errorf("invalid chat_id: %w", err)
	}
	title, err := idx.ChatAPI.LoadChatTitle(ctx, chatID)
	if err != nil {
		return err
	}
	return idx.Chats.UpsertChat(ctx, chatID, title)
}

func (idx *ChatSpaceIndexer) handleSpaceCreated(ctx context.Context, spaceRaw string) error {
	if idx.Spaces == nil || idx.SpaceAPI == nil {
		return fmt.Errorf("space indexer not configured")
	}
	spaceID, err := uuid.Parse(spaceRaw)
	if err != nil {
		return fmt.Errorf("invalid space_id: %w", err)
	}
	name, description, visibility, memberCount, err := idx.SpaceAPI.LoadSpace(ctx, spaceID)
	if err != nil {
		return err
	}
	if visibility == "private" {
		return idx.Spaces.DeleteSpace(ctx, spaceID)
	}
	return idx.Spaces.UpsertSpace(ctx, store.SpaceDocument{
		SpaceID:     spaceID,
		Name:        name,
		Description: description,
		Visibility:  visibility,
		MemberCount: memberCount,
	})
}
