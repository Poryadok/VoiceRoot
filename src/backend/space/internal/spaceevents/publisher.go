package spaceevents

import "context"

// Publisher publishes Space domain events to JetStream stream chat_events
// (subject space.created; logical stream chat.events per CONTRACT_MATRIX / space-service.md).
type Publisher interface {
	PublishSpaceCreated(ctx context.Context, spaceID, ownerProfileID string) error
	PublishTreeNodeUpserted(ctx context.Context, spaceID, nodeID, kind, chatID, voiceRoomID string) error
	PublishTreeNodeRemoved(ctx context.Context, spaceID, nodeID string) error
	PublishVoiceRoomCreated(ctx context.Context, spaceID, voiceRoomID string) error
	PublishVoiceRoomDeleted(ctx context.Context, spaceID, voiceRoomID string) error
	PublishInviteCreated(ctx context.Context, spaceID, inviteCode string) error
}
