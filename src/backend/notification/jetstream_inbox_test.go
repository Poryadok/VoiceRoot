package main

import (
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
)

func TestNotificationConsumerUsesScopedRequestInbox(t *testing.T) {
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)

	nc, _, err := connectNotificationConsumer(s.ClientURL(), "message")
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	require.True(t, strings.HasPrefix(nc.NewInbox(), "_INBOX.voice.notification."))
}
