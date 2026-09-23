// Package jetstreambind verifies centrally provisioned push consumers before binding.
package jetstreambind

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// Bind verifies every security-relevant consumer property before attaching the
// callback. It deliberately has no create, update, or delete capability.
func Bind(js nats.JetStreamContext, stream, durable, subject, deliverSubject string, handler nats.MsgHandler) (*nats.Subscription, error) {
	info, err := js.ConsumerInfo(stream, durable)
	if err != nil {
		return nil, fmt.Errorf("consumer info %s/%s: %w", stream, durable, err)
	}
	if err := verify(info, stream, durable, subject, deliverSubject); err != nil {
		return nil, fmt.Errorf("consumer %s/%s: %w", stream, durable, err)
	}
	return js.Subscribe(subject, handler, nats.Bind(stream, durable), nats.ManualAck())
}

func verify(info *nats.ConsumerInfo, stream, durable, subject, deliverSubject string) error {
	if info == nil || info.Stream != stream || info.Name != durable || info.Config.Durable != durable {
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

// Connect disables implicit reconnect so every reconnect starts a new run and
// must pass ConsumerInfo preflight before binding again.
func Connect(url, name string) (*nats.Conn, <-chan struct{}, error) {
	lost := make(chan struct{})
	var once sync.Once
	nc, err := nats.Connect(url, nats.Name(name), nats.Timeout(10*time.Second), nats.RetryOnFailedConnect(true), nats.MaxReconnects(0),
		nats.DisconnectErrHandler(func(*nats.Conn, error) { once.Do(func() { close(lost) }) }),
		nats.ClosedHandler(func(*nats.Conn) { once.Do(func() { close(lost) }) }),
	)
	return nc, lost, err
}

// Watch stops delivery on disconnect or later durable drift.
func Watch(ctx context.Context, lost <-chan struct{}, js nats.JetStreamContext, stream, durable, subject, deliverSubject string) error {
	return watch(ctx, lost, js, stream, durable, subject, deliverSubject, 5*time.Second)
}

func watch(ctx context.Context, lost <-chan struct{}, js nats.JetStreamContext, stream, durable, subject, deliverSubject string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-lost:
			return errors.New("NATS connection lost")
		case <-ticker.C:
			info, err := js.ConsumerInfo(stream, durable)
			if err != nil {
				return fmt.Errorf("consumer info %s/%s: %w", stream, durable, err)
			}
			if err := verify(info, stream, durable, subject, deliverSubject); err != nil {
				return err
			}
		}
	}
}
