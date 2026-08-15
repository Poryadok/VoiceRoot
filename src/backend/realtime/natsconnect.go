package main

import (
	"github.com/nats-io/nats.go"

	"voice/backend/pkg/runtimeconfig"
)

// natsConnectOptions returns shared client options for Realtime NATS consumers.
func natsConnectOptions(name string) []nats.Option {
	return []nats.Option{
		nats.Name(name),
		nats.Timeout(runtimeconfig.NATSConnectTimeoutFromEnv()),
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(runtimeconfig.NATSReconnectWaitFromEnv()),
	}
}
