package voiceevents

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/stretchr/testify/require"
)

func TestPublisherUsesVoiceReplyInbox(t *testing.T) {
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()})
	require.NoError(t, err)
	go srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)

	pub, err := NewJetStreamPublisher(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })
	require.Regexp(t, `^_INBOX\.voice\.voice\.`, pub.nc.NewRespInbox())
}
