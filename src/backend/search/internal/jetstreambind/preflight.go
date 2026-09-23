// Package jetstreambind verifies centrally provisioned push consumers before binding.
package jetstreambind

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// Bind verifies every security-relevant consumer property before attaching the
// callback. It deliberately has no create, update, or delete capability.
func Bind(js nats.JetStreamContext, stream, durable, subject, deliverSubject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	info, err := js.ConsumerInfo(stream, durable)
	if err != nil {
		return nil, fmt.Errorf("consumer info %s/%s: %w", stream, durable, err)
	}
	if err := verify(info, durable, subject, deliverSubject); err != nil {
		return nil, fmt.Errorf("consumer %s/%s: %w", stream, durable, err)
	}
	return js.Subscribe(subject, handler, nats.Bind(stream, durable), nats.ManualAck())
}

func verify(info *nats.ConsumerInfo, durable, subject, deliverSubject string) error {
	if info == nil || info.Name != durable || info.Config.Durable != durable {
		return fmt.Errorf("durable identity mismatch")
	}
	if len(info.Config.FilterSubjects) != 0 || info.Config.FilterSubject != subject {
		return fmt.Errorf("filter subject mismatch")
	}
	if info.Config.DeliverSubject != deliverSubject {
		return fmt.Errorf("deliver subject mismatch")
	}
	if info.Config.AckPolicy != nats.AckExplicitPolicy {
		return fmt.Errorf("ack policy mismatch")
	}
	if info.Config.DeliverPolicy != nats.DeliverNewPolicy {
		return fmt.Errorf("deliver policy mismatch")
	}
	return nil
}
