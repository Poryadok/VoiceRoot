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
	"voice/backend/role/internal/store"
)

const voicePolicyPollInterval = time.Second
const voicePolicyPageSize = 250

type voicePolicySource interface {
	ReadLatestVoicePolicyInvalidations(context.Context, uuid.UUID, int) ([]store.VoicePolicyInvalidation, error)
}

type voicePolicyPublisher interface {
	PublishVoiceRoomPolicyInvalidated(context.Context, *eventsv1.RoleStreamEvent) (*nats.PubAck, error)
}

// VoicePolicyDispatcher advances process-local source progress only after the
// immutable snapshot receives its JetStream PubAck. Voice owns reconciliation.
type VoicePolicyDispatcher struct {
	store     voicePolicySource
	publisher voicePolicyPublisher
	acked     map[uuid.UUID]uint64
	OnError   func(error)
}

func NewVoicePolicyDispatcher(source voicePolicySource, publisher voicePolicyPublisher) *VoicePolicyDispatcher {
	return &VoicePolicyDispatcher{store: source, publisher: publisher, acked: make(map[uuid.UUID]uint64)}
}

func (d *VoicePolicyDispatcher) DispatchOnce(ctx context.Context) error {
	if d == nil || d.store == nil || d.publisher == nil {
		return fmt.Errorf("voice policy dispatcher is not configured")
	}
	cursor := uuid.Nil
	for {
		rows, err := d.store.ReadLatestVoicePolicyInvalidations(ctx, cursor, voicePolicyPageSize)
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}
		for _, row := range rows {
			if row.SpaceID == uuid.Nil || row.EventID == uuid.Nil || row.Epoch == 0 || strings.Compare(row.SpaceID.String(), cursor.String()) <= 0 {
				return fmt.Errorf("invalid or unordered voice policy row")
			}
			if row.Epoch > d.acked[row.SpaceID] {
				envelope := &eventsv1.RoleStreamEvent{
					EventId: row.EventID.String(), OccurredAt: timestamppb.New(row.CreatedAt.UTC()),
					Payload: &eventsv1.RoleStreamEvent_VoiceRoomPolicyInvalidated{
						VoiceRoomPolicyInvalidated: &eventsv1.VoiceRoomPolicyInvalidated{
							SpaceId: row.SpaceID.String(), PolicyEpoch: row.Epoch,
							VoiceRoomId: optionalUUIDString(row.RoomID), ProfileId: optionalUUIDString(row.ProfileID),
						},
					},
				}
				ack, err := d.publisher.PublishVoiceRoomPolicyInvalidated(ctx, envelope)
				if err != nil {
					return err
				}
				if ack == nil || ack.Stream != "role_events" || ack.Sequence == 0 {
					return fmt.Errorf("voice policy invalidation lacks role_events publish acknowledgement")
				}
				d.acked[row.SpaceID] = row.Epoch
			}
			cursor = row.SpaceID
		}
		if len(rows) < voicePolicyPageSize {
			return nil
		}
	}
}

func (d *VoicePolicyDispatcher) Run(ctx context.Context) error {
	if d == nil {
		return fmt.Errorf("Role Voice policy dispatcher is not configured")
	}
	dispatch := func() {
		if err := d.DispatchOnce(ctx); err != nil && d.OnError != nil {
			d.OnError(err)
		}
	}
	dispatch()
	ticker := time.NewTicker(voicePolicyPollInterval)
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
