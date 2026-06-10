package roleevents

import "context"

// Publisher emits role domain events to NATS JetStream (role.events).
type Publisher interface {
	PublishRoleCreated(ctx context.Context, spaceID, roleID, name string) error
	PublishRoleUpdated(ctx context.Context, spaceID, roleID string, changedFields []string) error
	PublishRoleDeleted(ctx context.Context, spaceID, roleID string) error
	PublishRoleAssigned(ctx context.Context, spaceID, profileID, roleID string) error
	PublishRoleRevoked(ctx context.Context, spaceID, profileID, roleID string) error
	PublishChatOverrideSet(ctx context.Context, chatID, roleID string) error
	PublishVoiceOverrideSet(ctx context.Context, voiceRoomID, roleID string) error
}

// NoopPublisher discards events (tests / degraded mode).
type NoopPublisher struct{}

func (NoopPublisher) PublishRoleCreated(context.Context, string, string, string) error { return nil }
func (NoopPublisher) PublishRoleUpdated(context.Context, string, string, []string) error {
	return nil
}
func (NoopPublisher) PublishRoleDeleted(context.Context, string, string) error { return nil }
func (NoopPublisher) PublishRoleAssigned(context.Context, string, string, string) error {
	return nil
}
func (NoopPublisher) PublishRoleRevoked(context.Context, string, string, string) error { return nil }
func (NoopPublisher) PublishChatOverrideSet(context.Context, string, string) error     { return nil }
func (NoopPublisher) PublishVoiceOverrideSet(context.Context, string, string) error    { return nil }
