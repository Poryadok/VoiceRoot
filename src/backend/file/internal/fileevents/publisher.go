package fileevents

import "context"

// Publisher publishes file domain events to NATS JetStream (file.events).
type Publisher interface {
	PublishFileExpired(ctx context.Context, fileID string, chatID *string) error
	Close() error
}

// NoopPublisher drops events (tests / NATS optional).
type NoopPublisher struct{}

func (NoopPublisher) PublishFileExpired(context.Context, string, *string) error { return nil }
func (NoopPublisher) Close() error                                              { return nil }
