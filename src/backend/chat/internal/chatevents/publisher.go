package chatevents

import "context"

// Publisher publishes Chat domain events to JetStream stream chat_events
// (subjects chat.created, chat.member_changed; logical stream chat.events per CONTRACT_MATRIX).
type Publisher interface {
	PublishChatCreated(ctx context.Context, chatID, chatType string) error
	// PublishChatMemberChanged emits joined | left (see voice.events.v1.ChatMemberChanged.change).
	PublishChatMemberChanged(ctx context.Context, chatID, profileID, change string) error
}
