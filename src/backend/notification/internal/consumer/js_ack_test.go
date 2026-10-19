package consumer

import (
	"errors"
	"math"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

type retryAckSpy struct {
	ack, nak, term              int
	delay                       time.Duration
	delivered                   uint64
	metadataErr, dispositionErr error
}

func (s *retryAckSpy) Ack(...nats.AckOpt) error  { s.ack++; return s.dispositionErr }
func (s *retryAckSpy) Nak(...nats.AckOpt) error  { s.nak++; return s.dispositionErr }
func (s *retryAckSpy) Term(...nats.AckOpt) error { s.term++; return s.dispositionErr }
func (s *retryAckSpy) NakWithDelay(d time.Duration, _ ...nats.AckOpt) error {
	s.delay = d
	return s.dispositionErr
}
func (s *retryAckSpy) Metadata() (*nats.MsgMetadata, error) {
	return &nats.MsgMetadata{NumDelivered: s.delivered}, s.metadataErr
}

func TestNotificationTransientRetryBackoff(t *testing.T) {
	for _, tc := range []struct {
		name        string
		attempt     uint64
		metadataErr error
		want        time.Duration
	}{
		{"missing count", 0, nil, time.Second},
		{"first", 1, nil, time.Second},
		{"second", 2, nil, 2 * time.Second},
		{"third", 3, nil, 4 * time.Second},
		{"before cap", 9, nil, 256 * time.Second},
		{"cap", 10, nil, 5 * time.Minute},
		{"unsigned overflow", math.MaxUint64, nil, 5 * time.Minute},
		{"unavailable metadata", 8, errors.New("metadata unavailable"), time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spy := &retryAckSpy{delivered: tc.attempt, metadataErr: tc.metadataErr}
			JetStreamConsumeAck(spy, errors.New("temporary dependency failure"))
			require.Equal(t, tc.want, spy.delay)
			require.Zero(t, spy.ack)
			require.Zero(t, spy.nak, "no immediate NAK")
			require.Zero(t, spy.term, "no retry exhaustion discard")
		})
	}
	spy := &retryAckSpy{dispositionErr: errors.New("transport failed")}
	JetStreamConsumeAck(spy, errors.New("dependency unavailable"))
	require.Equal(t, time.Second, spy.delay)
	require.Zero(t, spy.nak, "failed delayed disposition must not spin")
	require.Zero(t, spy.ack)
	require.Zero(t, spy.term)
	spy = &retryAckSpy{}
	JetStreamConsumeAck(spy, nil)
	require.Equal(t, 1, spy.ack)
	require.Zero(t, spy.delay)
	JetStreamConsumeAck(nil, errors.New("unavailable"))
}

func TestNotificationTransientRetryRealJetStream(t *testing.T) {
	srv, err := natsserver.NewServer(&natsserver.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go srv.Start()
	t.Cleanup(func() { srv.Shutdown(); srv.WaitForShutdown() })
	require.True(t, srv.ReadyForConnections(10*time.Second))
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "notification_retry", Subjects: []string{"notification.retry"}})
	require.NoError(t, err)
	sub, err := js.PullSubscribe("notification.retry", "retry_proof", nats.ManualAck(), nats.AckWait(10*time.Second), nats.MaxDeliver(-1))
	require.NoError(t, err)
	_, err = js.Publish("notification.retry", []byte("immutable notification event"))
	require.NoError(t, err)
	first, err := sub.Fetch(1, nats.MaxWait(3*time.Second))
	require.NoError(t, err)
	started := time.Now()
	JetStreamConsumeAck(first[0], errors.New("temporary provider outage"))
	require.NoError(t, nc.Flush())
	_, err = sub.Fetch(1, nats.MaxWait(250*time.Millisecond))
	require.ErrorIs(t, err, nats.ErrTimeout, "temporary error must not spin immediately")
	info, err := sub.ConsumerInfo()
	require.NoError(t, err)
	require.EqualValues(t, 1, info.NumAckPending, "failed event remains pending")
	second, err := sub.Fetch(1, nats.MaxWait(4*time.Second))
	require.NoError(t, err)
	require.GreaterOrEqual(t, time.Since(started), 900*time.Millisecond)
	require.Equal(t, first[0].Data, second[0].Data)
	metadata, err := second[0].Metadata()
	require.NoError(t, err)
	require.EqualValues(t, 2, metadata.NumDelivered)
	JetStreamConsumeAck(second[0], nil)
	require.NoError(t, nc.Flush())
	require.Eventually(t, func() bool { ci, e := sub.ConsumerInfo(); return e == nil && ci.NumAckPending == 0 }, 3*time.Second, 10*time.Millisecond)
}
