package messageevents

import "context"

// MessageEventsPublisher publishes Messaging domain events to JetStream stream message_events
// (subjects message.sent, message.edited, message.deleted; logical stream message.events in CONTRACT_MATRIX).
type MessageEventsPublisher interface {
	PublishMessageSent(ctx context.Context, messageID, chatID, senderProfileID string, hasMentions bool, threadParentID string, isE2E bool, contentType string, sendSilent bool) error
	PublishGameEventMessageSent(ctx context.Context, messageID, chatID, senderProfileID, applicationID, environmentID string) error
	PublishMentionAdded(ctx context.Context, messageID, chatID, senderProfileID string, mentionedProfileIDs []string, sendSilent bool, gameApplicationID, gameEnvironmentID string) error
	PublishMessageEdited(ctx context.Context, messageID, chatID string, isE2E bool) error
	PublishMessageDeleted(ctx context.Context, messageID, chatID string) error
	PublishMessageRead(ctx context.Context, messageID, chatID, profileID string) error
	PublishReactionAdded(ctx context.Context, messageID, chatID, profileID, messageAuthorProfileID, emoji, gameApplicationID, gameEnvironmentID string) error
	PublishReactionRemoved(ctx context.Context, messageID, chatID, profileID, emoji string) error
	PublishMessagePinned(ctx context.Context, messageID, chatID, pinnedBy string) error
	PublishMessageUnpinned(ctx context.Context, messageID, chatID, unpinnedBy string) error
	PublishMessageForwarded(ctx context.Context, messageID, sourceChatID, targetChatID, forwarderProfileID string) error
}
