package mmevents

import (
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPublisherUsesScopedNATSInbox(t *testing.T) {
	var opts nats.Options
	for _, option := range publisherNATSOptions() {
		require.NoError(t, option(&opts))
	}
	require.Equal(t, "_INBOX.voice.matchmaking", opts.InboxPrefix)
}
