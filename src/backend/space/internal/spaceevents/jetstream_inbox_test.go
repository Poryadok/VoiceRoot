package spaceevents

import (
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPublisherUsesScopedNATSInbox(t *testing.T) {
	var opts nats.Options
	for _, option := range spaceEventsNATSOptions() {
		require.NoError(t, option(&opts))
	}
	require.Equal(t, "_INBOX.voice.space", opts.InboxPrefix)
}
