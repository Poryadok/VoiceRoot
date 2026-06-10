package spaceevents

import "context"

// Publisher publishes Space domain events to JetStream stream chat_events
// (subject space.created; logical stream chat.events per CONTRACT_MATRIX / space-service.md).
type Publisher interface {
	PublishSpaceCreated(ctx context.Context, spaceID, ownerProfileID string) error
}
