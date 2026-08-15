package consumer

import "github.com/nats-io/nats.go"

type jetStreamMsgAck interface {
	Ack(...nats.AckOpt) error
	Nak(...nats.AckOpt) error
	Term(...nats.AckOpt) error
}

// JetStreamConsumeAck acks on success and naks on handler error for redelivery.
func JetStreamConsumeAck(msg jetStreamMsgAck, handlerErr error) {
	if msg == nil {
		return
	}
	if handlerErr != nil {
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

// JetStreamTermAck permanently drops poison messages that cannot be decoded.
func JetStreamTermAck(msg jetStreamMsgAck) {
	if msg == nil {
		return
	}
	_ = msg.Term()
}

var _ jetStreamMsgAck = (*nats.Msg)(nil)
