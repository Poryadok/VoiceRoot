package main

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestRealtimeNATSConnectionsUseScopedRequestInbox(t *testing.T) {
	options := &nats.Options{}
	for _, option := range natsConnectOptions("voice-realtime-test") {
		require.NoError(t, option(options))
	}
	require.Equal(t, "_INBOX.voice.realtime", options.InboxPrefix)
}
