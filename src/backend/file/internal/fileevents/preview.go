package fileevents

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/pkg/filepreview"
)

func (p *JetStreamPublisher) PublishPreviewEvent(ctx context.Context, payload []byte) error {
	var e eventsv1.FileStreamEvent
	if len(payload) > 8192 || proto.Unmarshal(payload, &e) != nil {
		return errors.New("invalid preview outbox payload")
	}
	id, err := uuid.Parse(e.EventId)
	if err != nil || id == uuid.Nil {
		return errors.New("invalid preview event ID")
	}
	if err = filepreview.ValidateEvent(e.GetFileProcessed()); err != nil {
		return err
	}
	return p.publishProto(ctx, subjectFileProcessed, &e)
}
