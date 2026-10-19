package consumer

import (
	"time"

	"github.com/nats-io/nats.go"
)

type jetStreamMsgAck interface {
	Ack(...nats.AckOpt) error
	NakWithDelay(time.Duration, ...nats.AckOpt) error
	Term(...nats.AckOpt) error
}

// JetStreamConsumeAck acks on success and schedules transient failures without
// blocking the callback or imposing a delivery-attempt cap.
func JetStreamConsumeAck(msg jetStreamMsgAck, handlerErr error) {
	if msg == nil {
		return
	}
	if handlerErr != nil {
		_ = msg.NakWithDelay(notificationRetryDelay(msg))
		return
	}
	_ = msg.Ack()
}

func notificationRetryDelay(msg jetStreamMsgAck) time.Duration {
	const maxDelay = 5 * time.Minute
	metadataMsg, ok := msg.(interface {
		Metadata() (*nats.MsgMetadata, error)
	})
	if !ok {
		return time.Second
	}
	metadata, err := metadataMsg.Metadata()
	if err != nil || metadata == nil || metadata.NumDelivered == 0 {
		return time.Second
	}
	// Saturate before shifting or converting the unsigned broker counter.
	if metadata.NumDelivered >= 10 {
		return maxDelay
	}
	return time.Second << (metadata.NumDelivered - 1)
}

// JetStreamTermAck permanently drops poison messages that cannot be decoded.
func JetStreamTermAck(msg jetStreamMsgAck) {
	if msg == nil {
		return
	}
	_ = msg.Term()
}

var _ jetStreamMsgAck = (*nats.Msg)(nil)
