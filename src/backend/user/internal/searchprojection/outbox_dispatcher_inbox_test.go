package searchprojection

import (
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestProjectionDispatcherUsesScopedNATSInbox(t *testing.T) {
	var opts nats.Options
	for _, option := range projectionNATSOptions() {
		require.NoError(t, option(&opts))
	}
	require.Equal(t, "_INBOX.voice.user", opts.InboxPrefix)
}
