package fileevents

import "context"

// Publisher publishes file domain events to NATS JetStream (file.events).
type Publisher interface {
	PublishFileUploaded(ctx context.Context, fileID, uploaderProfileID string) error
	PublishFileProcessed(ctx context.Context, fileID, status, convertedR2Key, thumbnailR2Key string) error
	PublishFileScanInfected(ctx context.Context, fileID, uploaderProfileID string) error
	PublishFileExpired(ctx context.Context, fileID string, chatID *string) error
	Close() error
}

// NoopPublisher drops events (tests / NATS optional).
type NoopPublisher struct{}

func (NoopPublisher) PublishFileUploaded(context.Context, string, string) error { return nil }
func (NoopPublisher) PublishFileProcessed(context.Context, string, string, string, string) error {
	return nil
}
func (NoopPublisher) PublishFileScanInfected(context.Context, string, string) error { return nil }
func (NoopPublisher) PublishFileExpired(context.Context, string, *string) error     { return nil }
func (NoopPublisher) Close() error                                                  { return nil }
