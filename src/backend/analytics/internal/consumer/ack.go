package consumer

import "github.com/nats-io/nats.go"

// jetStreamMsgAck supports JetStream ack/nak without test doubles.
type jetStreamMsgAck interface {
	Ack(...nats.AckOpt) error
	Nak(...nats.AckOpt) error
}

func jetStreamConsumeAck(msg jetStreamMsgAck, handlerErr error) {
	if msg == nil {
		return
	}
	if handlerErr != nil {
		_ = msg.Nak()
		return
	}
}

var _ jetStreamMsgAck = (*nats.Msg)(nil)
