package storyevents

import "context"

// Publisher publishes story domain events to NATS JetStream (story.events).
type Publisher interface {
	PublishStoryCreated(ctx context.Context, storyID, authorProfileID, storyType, gameTag string, mentionProfileIDs []string) error
	PublishStoryViewed(ctx context.Context, storyID, viewerProfileID string) error
	PublishStoryReacted(ctx context.Context, storyID, reactorProfileID, emoji string) error
	PublishStoryExpired(ctx context.Context, storyID string) error
	PublishStoryHighlightCreated(ctx context.Context, highlightID, profileID string) error
	PublishStoryLfpCreated(ctx context.Context, storyID, authorProfileID, criteriaJSON string) error
	Close() error
}

// NoopPublisher drops events (tests / NATS optional).
type NoopPublisher struct{}

func (NoopPublisher) PublishStoryCreated(context.Context, string, string, string, string, []string) error {
	return nil
}
func (NoopPublisher) PublishStoryViewed(context.Context, string, string) error { return nil }
func (NoopPublisher) PublishStoryReacted(context.Context, string, string, string) error {
	return nil
}
func (NoopPublisher) PublishStoryExpired(context.Context, string) error { return nil }
func (NoopPublisher) PublishStoryHighlightCreated(context.Context, string, string) error {
	return nil
}
func (NoopPublisher) PublishStoryLfpCreated(context.Context, string, string, string) error {
	return nil
}
func (NoopPublisher) Close() error { return nil }
