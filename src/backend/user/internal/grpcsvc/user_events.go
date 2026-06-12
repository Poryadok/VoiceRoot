package grpcsvc

import "context"

// UserEventsPublisher emits user.events JetStream payloads.
type UserEventsPublisher interface {
	PublishProfileCreated(ctx context.Context, profileID, accountID string) error
	PublishProfileUpdated(ctx context.Context, profileID, accountID, changedFieldsJSON string) error
}
