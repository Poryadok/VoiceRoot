package messageevents

import "context"

// MessageEventsPublisher publishes Messaging domain events to JetStream stream message_events
// (subjects message.sent, message.edited, message.deleted; logical stream message.events in CONTRACT_MATRIX).
type MessageEventsPublisher interface {
	PublishMessageSent(ctx context.Context, messageID, chatID, senderProfileID string) error
	PublishMessageEdited(ctx context.Context, messageID, chatID string) error
	PublishMessageDeleted(ctx context.Context, messageID, chatID string) error
	PublishMessageRead(ctx context.Context, messageID, chatID, profileID string) error
}
