package main

import (
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
)

// realtimeConsumerDeliverSubject is the bootstrap-owned push target for one
// Realtime instance. Keep this in lockstep with docker/nats/realtime-bootstrap.sh.
func realtimeConsumerDeliverSubject(instanceID, consumer string) string {
	id := strings.TrimSpace(instanceID)
	if id == "" {
		id = "unknown"
	}
	return "_INBOX.voice." + strings.ReplaceAll(id, "-", "") + "." + consumer
}

// validateRealtimeConsumerConfig prevents a bind-only subscription from
// attaching to a pre-existing durable whose scope or delivery semantics drifted.
func validateRealtimeConsumerConfig(js nats.JetStreamContext, stream, durable, filter, deliverSubject string) error {
	info, err := js.ConsumerInfo(stream, durable)
	if err != nil {
		return fmt.Errorf("inspect pre-provisioned %s consumer %q: %w", stream, durable, err)
	}
	config := info.Config
	if config.FilterSubject != filter || len(config.FilterSubjects) != 0 ||
		config.DeliverSubject != deliverSubject ||
		config.AckPolicy != nats.AckExplicitPolicy ||
		config.DeliverPolicy != nats.DeliverNewPolicy {
		return fmt.Errorf("pre-provisioned %s consumer %q has incompatible configuration", stream, durable)
	}
	return nil
}
