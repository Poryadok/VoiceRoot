package indexer

import "github.com/nats-io/nats.go"

// jetStreamMsgAck supports JetStream ack/nak/term without importing test doubles.
type jetStreamMsgAck interface {
	Ack(...nats.AckOpt) error
	Nak(...nats.AckOpt) error
	Term(...nats.AckOpt) error
}

// jetStreamConsumeAck applies JetStream delivery disposition after handler completion.
// Handler errors are nacked for redelivery; success is acked.
func jetStreamConsumeAck(msg jetStreamMsgAck, handlerErr error) {
	if msg == nil {
		return
	}
	if handlerErr != nil {
		_ = msg.Nak()
		return
	}
	_ = msg.Ack()
}

// jetStreamTermAck permanently drops a poison message that cannot be processed.
func jetStreamTermAck(msg jetStreamMsgAck) {
	if msg == nil {
		return
	}
	_ = msg.Term()
}

// ensure *nats.Msg implements jetStreamMsgAck at compile time.
var _ jetStreamMsgAck = (*nats.Msg)(nil)
