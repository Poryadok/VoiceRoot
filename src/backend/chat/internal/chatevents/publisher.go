package chatevents

import (
	"context"

	eventsv1 "voice.app/voice/events/v1"
)

// Publisher publishes Chat domain events to JetStream stream chat_events
// (logical stream chat.events per CONTRACT_MATRIX).
type Publisher interface {
	PublishChatCreated(ctx context.Context, chatID, chatType string) error
	PublishChatUpdated(ctx context.Context, chatID string, changedFields []string) error
	// The persisted envelope carries its stable purge identity and timestamp; EventId is also the
	// JetStream deduplication key.
	PublishChatDeleted(ctx context.Context, event *eventsv1.ChatStreamEvent) error
	// PublishChatMemberChanged emits the canonical complete change union documented by Chat service.
	PublishChatMemberChanged(ctx context.Context, chatID, profileID, change string) error
}
