package moderationevents

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

func startJSTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := &server.Options{
		Host:      "127.0.0.1",
		Port:      -1,
		NoLog:     true,
		NoSigs:    true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	}
	s, err := server.NewServer(opts)
	require.NoError(t, err)
	go s.Start()
	if !s.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}
	t.Cleanup(func() { s.Shutdown() })
	return s
}

func TestJetStreamPublisher_SanctionAppliedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectSanctionApplied)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const (
		sanctionID = "33333333-3333-4333-8333-333333333333"
		accountID  = "44444444-4444-4444-8444-444444444444"
	)
	require.NoError(t, pub.PublishSanctionApplied(ctx, sanctionID, accountID, "temp_ban"))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.ModerationStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	applied := env.GetSanctionApplied()
	require.NotNil(t, applied)
	require.Equal(t, sanctionID, applied.GetSanctionId())
	require.Equal(t, accountID, applied.GetTargetAccountId())
	require.Equal(t, "temp_ban", applied.GetType())
}
