package grpcsvc

import "context"

// UserEventsPublisher emits user.events JetStream payloads.
type UserEventsPublisher interface {
	PublishProfileCreated(ctx context.Context, profileID, accountID string) error
	PublishProfileUpdated(ctx context.Context, profileID string, changedFields []string) error
	PublishProfileSwitched(ctx context.Context, accountID, oldProfileID, newProfileID string) error
	PublishVerified(ctx context.Context, profileID, verificationType string) error
	PublishPresenceChanged(ctx context.Context, profileID, oldStatus, newStatus string) error
	PublishGameDetected(ctx context.Context, profileID, gameName string) error
	PublishSettingsChanged(ctx context.Context, profileID string, changedKeys []string, changedKeysJSON string) error
}
