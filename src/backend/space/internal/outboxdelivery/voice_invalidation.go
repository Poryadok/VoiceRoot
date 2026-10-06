package outboxdelivery

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/types/known/timestamppb"

	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/store"
)

const voiceInvalidationPollInterval = time.Second

type voiceInvalidationSource interface {
	ReadLatestVoiceAccessInvalidations(context.Context, uuid.UUID, int) ([]store.VoiceAccessInvalidation, error)
}

type voiceInvalidationPublisher interface {
	PublishVoiceRoomAccessInvalidated(context.Context, *eventsv1.ChatStreamEvent) (*nats.PubAck, error)
}

// VoiceInvalidationDispatcher publishes the newest immutable snapshot for
// each Space. Its in-memory high-water is only PubAck source progress; a
// restart replays the latest row and the Voice consumer reconciles current state.
type VoiceInvalidationDispatcher struct {
	store     voiceInvalidationSource
	publisher voiceInvalidationPublisher
	acked     map[uuid.UUID]uint64
	OnError   func(error)
}

func NewVoiceInvalidationDispatcher(source voiceInvalidationSource, publisher voiceInvalidationPublisher) *VoiceInvalidationDispatcher {
	return &VoiceInvalidationDispatcher{store: source, publisher: publisher, acked: make(map[uuid.UUID]uint64)}
}

func (d *VoiceInvalidationDispatcher) DispatchOnce(ctx context.Context) error {
	if d == nil || d.store == nil || d.publisher == nil {
		return fmt.Errorf("Space Voice invalidation dispatcher is not configured")
	}
	cursor := uuid.Nil
	for {
		rows, err := d.store.ReadLatestVoiceAccessInvalidations(ctx, cursor, storeVoiceInvalidationPageSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if row.SpaceID == uuid.Nil || row.EventID == uuid.Nil || row.Epoch == 0 || strings.Compare(row.SpaceID.String(), cursor.String()) <= 0 {
				return fmt.Errorf("invalid or unordered Space Voice invalidation row")
			}
			if row.Epoch > d.acked[row.SpaceID] {
				envelope := &eventsv1.ChatStreamEvent{
					EventId: row.EventID.String(), OccurredAt: timestamppb.New(row.CreatedAt.UTC()),
					Payload: &eventsv1.ChatStreamEvent_VoiceRoomAccessInvalidated{
						VoiceRoomAccessInvalidated: &eventsv1.VoiceRoomAccessInvalidated{
							SpaceId: row.SpaceID.String(), AccessEpoch: row.Epoch,
							VoiceRoomId: optionalUUIDString(row.RoomID), ProfileId: optionalUUIDString(row.ProfileID),
						},
					},
				}
				ack, err := d.publisher.PublishVoiceRoomAccessInvalidated(ctx, envelope)
				if err != nil {
					return err
				}
				if ack == nil || ack.Stream != "chat_events" || ack.Sequence == 0 {
					return fmt.Errorf("Space Voice invalidation lacks chat_events PubAck")
				}
				d.acked[row.SpaceID] = row.Epoch
			}
			cursor = row.SpaceID
		}
		if len(rows) < storeVoiceInvalidationPageSize {
			return nil
		}
	}
}

const storeVoiceInvalidationPageSize = 250

func (d *VoiceInvalidationDispatcher) Run(ctx context.Context) error {
	if d == nil {
		return fmt.Errorf("Space Voice invalidation dispatcher is not configured")
	}
	dispatch := func() {
		if err := d.DispatchOnce(ctx); err != nil && d.OnError != nil {
			d.OnError(err)
		}
	}
	dispatch()
	ticker := time.NewTicker(voiceInvalidationPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			dispatch()
		}
	}
}

func optionalUUIDString(value *uuid.UUID) *string {
	if value == nil {
		return nil
	}
	encoded := value.String()
	return &encoded
}
