package main

import (
	"fmt"

	"github.com/nats-io/nats.go"
)

// consumerNATSConnection borrows the process connection when provided. The
// consumer still owns its subscriptions, but only main may drain that transport.
// Standalone consumers retain their own connection for tools and focused tests.
func consumerNATSConnection(url, name string, shared ...*nats.Conn) (*nats.Conn, func(), error) {
	if len(shared) > 0 {
		if len(shared) != 1 || shared[0] == nil || shared[0].IsClosed() {
			return nil, nil, fmt.Errorf("shared NATS connection unavailable")
		}
		return shared[0], func() {}, nil
	}
	nc, err := nats.Connect(url, natsConnectOptions(name)...)
	if err != nil {
		return nil, nil, err
	}
	return nc, func() { _ = nc.Drain() }, nil
}
