package outboxdelivery

import (
	"context"
	"errors"
	"math"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/space/internal/store"
)

type MemberAdmissionEventStore interface {
	ListPendingMemberAdmissionEvents(context.Context, int) ([]store.MemberAdmissionEvent, error)
	MarkMemberAdmissionEventDelivered(context.Context, store.MemberAdmissionEvent, string, uint64) error
}

// MemberAdmissionDispatcher always reuses the DB-issued event identity/body.
// Network publication runs outside Space transactions; only a valid PubAck can
// advance the durable outbox. Restart/duplicate publication is safe to replay.
type MemberAdmissionDispatcher struct {
	Store     MemberAdmissionEventStore
	Publisher Transport
}

func (d *MemberAdmissionDispatcher) DispatchOnce(ctx context.Context) error {
	if d == nil || d.Store == nil || d.Publisher == nil {
		return errors.New("member admission dispatcher unavailable")
	}
	rows, err := d.Store.ListPendingMemberAdmissionEvents(ctx, 100)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.EventID == uuid.Nil || row.OperationID == uuid.Nil || row.SpaceID == uuid.Nil || row.ProfileID == uuid.Nil || row.Generation == 0 || row.Generation > math.MaxInt64 || row.OccurredAt.IsZero() {
			return errors.New("invalid member admission event")
		}
		stamp := timestamppb.New(row.OccurredAt.UTC())
		if err := stamp.CheckValid(); err != nil {
			return errors.New("invalid member admission event time")
		}
		envelope := &eventsv1.ChatStreamEvent{EventId: row.EventID.String(), OccurredAt: stamp, Payload: &eventsv1.ChatStreamEvent_SpaceMemberJoined{SpaceMemberJoined: &eventsv1.SpaceMemberJoined{SpaceId: row.SpaceID.String(), ProfileId: row.ProfileID.String()}}}
		body, err := proto.MarshalOptions{Deterministic: true}.Marshal(envelope)
		if err != nil {
			return err
		}
		msg := &nats.Msg{Subject: "space.member_joined", Data: body, Header: nats.Header{}}
		msg.Header.Set(nats.MsgIdHdr, row.EventID.String())
		ack, err := d.Publisher.Publish(ctx, msg)
		if err != nil {
			return err
		}
		if ack == nil || ack.Stream != "chat_events" || ack.Sequence == 0 || ack.Sequence > math.MaxInt64 {
			return errors.New("member admission lacks chat_events PubAck")
		}
		if err := d.Store.MarkMemberAdmissionEventDelivered(ctx, row, ack.Stream, ack.Sequence); err != nil {
			return err
		}
	}
	return nil
}
