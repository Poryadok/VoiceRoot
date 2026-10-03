package spaceevents

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/spacecore"
)

func (p *JetStreamPublisher) PublishLifecycle(ctx context.Context, record spacecore.LifecycleOutboxRecord, purgeDecidedAt time.Time) error {
	envelope, err := lifecycleEnvelope(record, purgeDecidedAt)
	if err != nil {
		return err
	}
	raw, err := (proto.MarshalOptions{Deterministic: true}).Marshal(envelope)
	if err != nil {
		return err
	}
	message := &nats.Msg{Subject: record.EventType, Data: raw, Header: nats.Header{nats.MsgIdHdr: []string{record.EventID}}}
	ack, err := p.Publish(ctx, message)
	if err != nil {
		return err
	}
	if ack == nil || ack.Stream != streamName || ack.Sequence == 0 {
		return errors.New("invalid lifecycle JetStream acknowledgement")
	}
	return nil
}
func lifecycleEnvelope(r spacecore.LifecycleOutboxRecord, decidedAt time.Time) (*eventsv1.ChatStreamEvent, error) {
	for _, raw := range []string{r.EventID, r.SpaceID, r.DeletionOperationID} {
		id, err := uuid.Parse(raw)
		if err != nil || id == uuid.Nil || id.String() != raw {
			return nil, errors.New("invalid lifecycle event binding")
		}
	}
	if r.Generation == 0 || r.OccurredAt.IsZero() {
		return nil, errors.New("invalid lifecycle event timestamp")
	}
	envelope := &eventsv1.ChatStreamEvent{EventId: r.EventID, OccurredAt: timestamppb.New(r.OccurredAt)}
	switch r.EventType {
	case "space.deletion_scheduled":
		envelope.Payload = &eventsv1.ChatStreamEvent_SpaceDeletionScheduled{SpaceDeletionScheduled: &eventsv1.SpaceDeletionScheduled{ProtocolVersion: 1, SpaceId: r.SpaceID, DeletionOperationId: r.DeletionOperationID, Generation: r.Generation, ScheduledAt: timestamppb.New(r.OccurredAt), PurgeAfter: timestamppb.New(r.OccurredAt.Add(7 * 24 * time.Hour))}}
	case "space.restored":
		envelope.Payload = &eventsv1.ChatStreamEvent_SpaceRestored{SpaceRestored: &eventsv1.SpaceRestored{ProtocolVersion: 1, SpaceId: r.SpaceID, DeletionOperationId: r.DeletionOperationID, Generation: r.Generation, RestoredAt: timestamppb.New(r.OccurredAt)}}
	case "space.deleted":
		if decidedAt.IsZero() || decidedAt.After(r.OccurredAt) {
			return nil, errors.New("invalid lifecycle purge decision timestamp")
		}
		envelope.Payload = &eventsv1.ChatStreamEvent_SpaceDeleted{SpaceDeleted: &eventsv1.SpaceDeleted{ProtocolVersion: 1, SpaceId: r.SpaceID, DeletionOperationId: r.DeletionOperationID, Generation: r.Generation, PurgeDecidedAt: timestamppb.New(decidedAt), PurgedAt: timestamppb.New(r.OccurredAt)}}
	default:
		return nil, errors.New("invalid lifecycle event type")
	}
	return envelope, nil
}
