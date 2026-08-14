package fileevents

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/correlation"
	"voice/backend/pkg/natslog"
)

const (
	streamName            = "file_events"
	subjectFileUploaded   = "file.uploaded"
	subjectFileProcessed  = "file.processed"
	subjectFileScanInfected = "file.scan_infected"
	subjectFileExpired    = "file.expired"
)

// JetStreamPublisher publishes FileStreamEvent payloads to NATS JetStream.
type JetStreamPublisher struct {
	nc *nats.Conn
	js nats.JetStreamContext
	// Logger emits structured nats_publish lines; optional.
	Logger *slog.Logger

	ensureOnce sync.Once
	ensureErr  error
}

// NewJetStreamPublisher connects to NATS_URL and prepares JetStream handle.
func NewJetStreamPublisher(natsURL string) (*JetStreamPublisher, error) {
	if natsURL == "" {
		return nil, fmt.Errorf("empty NATS URL")
	}
	nc, err := nats.Connect(natsURL,
		nats.Name("voice-file-file-events"),
		nats.Timeout(10*time.Second),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		_ = nc.Drain()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &JetStreamPublisher{nc: nc, js: js}, nil
}

func fileEventStreamSubjects() []string {
	return []string{
		subjectFileUploaded,
		subjectFileProcessed,
		subjectFileScanInfected,
		subjectFileExpired,
	}
}

func (p *JetStreamPublisher) ensureStream() error {
	if p == nil || p.js == nil {
		return fmt.Errorf("jetstream publisher not initialized")
	}
	p.ensureOnce.Do(func() {
		desired := fileEventStreamSubjects()
		info, err := p.js.StreamInfo(streamName)
		if err != nil {
			_, p.ensureErr = p.js.AddStream(&nats.StreamConfig{
				Name:      streamName,
				Subjects:  desired,
				Retention: nats.LimitsPolicy,
				MaxAge:    7 * 24 * time.Hour,
				Storage:   nats.FileStorage,
			})
			return
		}
		existing := make(map[string]struct{}, len(info.Config.Subjects))
		for _, subject := range info.Config.Subjects {
			existing[subject] = struct{}{}
		}
		merged := append([]string(nil), info.Config.Subjects...)
		for _, subject := range desired {
			if _, ok := existing[subject]; ok {
				continue
			}
			merged = append(merged, subject)
		}
		if len(merged) == len(info.Config.Subjects) {
			return
		}
		cfg := info.Config
		cfg.Subjects = merged
		_, p.ensureErr = p.js.UpdateStream(&cfg)
	})
	return p.ensureErr
}

func (p *JetStreamPublisher) publishProto(ctx context.Context, subject string, env *eventsv1.FileStreamEvent) error {
	if err := p.ensureStream(); err != nil {
		return err
	}
	b, err := proto.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal FileStreamEvent: %w", err)
	}
	requestID := correlation.FromGRPC(ctx)
	msg := &nats.Msg{Subject: subject, Data: b, Header: nats.Header{}}
	natslog.SetRequestIDHeader(msg.Header, requestID)
	if _, err := p.js.PublishMsg(msg); err != nil {
		return fmt.Errorf("jetstream publish %s: %w", subject, err)
	}
	attrs := []slog.Attr{slog.String("event_id", env.GetEventId())}
	if uploaded := env.GetFileUploaded(); uploaded != nil {
		attrs = append(attrs, slog.String("file_id", uploaded.GetFileId()))
	}
	if processed := env.GetFileProcessed(); processed != nil {
		attrs = append(attrs, slog.String("file_id", processed.GetFileId()))
	}
	if scan := env.GetFileScanResult(); scan != nil {
		attrs = append(attrs, slog.String("file_id", scan.GetFileId()))
	}
	if expired := env.GetFileExpired(); expired != nil {
		attrs = append(attrs, slog.String("file_id", expired.GetFileId()))
	}
	natslog.LogPublish(p.Logger, subject, requestID, "file event published", attrs...)
	return nil
}

func newFileEvent() *eventsv1.FileStreamEvent {
	return &eventsv1.FileStreamEvent{
		EventId:    uuid.NewString(),
		OccurredAt: timestamppb.New(time.Now().UTC()),
	}
}

// PublishFileUploaded implements Publisher.
func (p *JetStreamPublisher) PublishFileUploaded(ctx context.Context, fileID, uploaderProfileID string) error {
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileUploaded{
		FileUploaded: &eventsv1.FileUploaded{
			FileId:             fileID,
			UploaderProfileId: uploaderProfileID,
		},
	}
	return p.publishProto(ctx, subjectFileUploaded, env)
}

// PublishFileProcessed implements Publisher.
func (p *JetStreamPublisher) PublishFileProcessed(ctx context.Context, fileID, status, convertedR2Key, thumbnailR2Key string) error {
	processed := &eventsv1.FileProcessed{
		FileId: fileID,
		Status: status,
	}
	if convertedR2Key != "" {
		processed.ConvertedR2Key = &convertedR2Key
	}
	if thumbnailR2Key != "" {
		processed.ThumbnailR2Key = &thumbnailR2Key
	}
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileProcessed{FileProcessed: processed}
	return p.publishProto(ctx, subjectFileProcessed, env)
}

// PublishFileScanInfected implements Publisher.
func (p *JetStreamPublisher) PublishFileScanInfected(ctx context.Context, fileID, uploaderProfileID string) error {
	uploader := uploaderProfileID
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileScanResult{
		FileScanResult: &eventsv1.FileScanResult{
			FileId:             fileID,
			Result:             "infected",
			UploaderProfileId: &uploader,
		},
	}
	return p.publishProto(ctx, subjectFileScanInfected, env)
}

// PublishFileExpired implements Publisher.
func (p *JetStreamPublisher) PublishFileExpired(ctx context.Context, fileID string, chatID *string) error {
	expired := &eventsv1.FileExpired{FileId: fileID}
	if chatID != nil && *chatID != "" {
		expired.ChatId = chatID
	}
	env := newFileEvent()
	env.Payload = &eventsv1.FileStreamEvent_FileExpired{FileExpired: expired}
	return p.publishProto(ctx, subjectFileExpired, env)
}

// Close drains the underlying NATS connection.
func (p *JetStreamPublisher) Close() error {
	if p == nil || p.nc == nil {
		return nil
	}
	return p.nc.Drain()
}
