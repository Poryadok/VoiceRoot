package spaceevents

import "context"

// Publisher publishes Space domain events to JetStream stream chat_events
// (subject space.created; logical stream chat.events per CONTRACT_MATRIX / space-service.md).
type Publisher interface {
	PublishSpaceCreated(ctx context.Context, spaceID, ownerProfileID string) error
	PublishTreeNodeUpserted(ctx context.Context, spaceID, nodeID, kind, chatID, voiceRoomID string, isPinned bool, pinOrder *int32) error
	PublishTreeNodeRemoved(ctx context.Context, spaceID, nodeID string) error
	PublishVoiceRoomCreated(ctx context.Context, spaceID, voiceRoomID string) error
	PublishVoiceRoomDeleted(ctx context.Context, spaceID, voiceRoomID string) error
	PublishInviteCreated(ctx context.Context, spaceID, inviteCode string) error
	PublishMemberJoined(ctx context.Context, spaceID, profileID string) error
	PublishMemberLeft(ctx context.Context, spaceID, profileID string) error
	PublishSpaceUpdated(ctx context.Context, spaceID string) error
	PublishSpaceDeleted(ctx context.Context, spaceID string) error
}
