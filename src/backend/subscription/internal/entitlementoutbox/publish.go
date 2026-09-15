package entitlementoutbox

import (
	"bytes"
	"context"
	"crypto/sha256"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
	eventsv1 "voice.app/voice/events/v1"
)

// Publish sends only stored, validated bytes. It does not allocate event IDs,
// configure a stream, or mark delivery complete. The owner records PubAck using
// Complete; a crash between these calls safely resends this same ID and payload.
func Publish(ctx context.Context, js nats.JetStreamContext, d Delivery) (Ack, error) {
	hash := sha256.Sum256(d.Payload)
	if !bytes.Equal(hash[:], d.PayloadHash) {
		return Ack{}, ErrContractMismatch
	}
	var e eventsv1.SubscriptionStreamEvent
	if err := proto.Unmarshal(d.Payload, &e); err != nil {
		return Ack{}, ErrContractMismatch
	}
	if err := Validate(&e); err != nil {
		return Ack{}, err
	}
	if d.EventID != e.EventId {
		return Ack{}, ErrContractMismatch
	}
	msg := nats.NewMsg(Subject)
	msg.Data = d.Payload
	msg.Header.Set(nats.MsgIdHdr, d.EventID)
	ack, err := js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		return Ack{}, err
	}
	if ack == nil || ack.Stream != "subscription_events" || ack.Sequence == 0 {
		return Ack{}, ErrContractMismatch
	}
	return Ack{Stream: ack.Stream, Sequence: ack.Sequence}, nil
}
