package rosterconsume

import (
	"context"
	"errors"
	"fmt"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"net"
	"sync/atomic"
	"testing"
	"time"
	"voice/backend/matchmaking/internal/store"
)

func TestValidateRosterDurableRejectsAuthorityAndDeliveryDrift(t *testing.T) {
	fixture := func() *nats.ConsumerInfo {
		return &nats.ConsumerInfo{Stream: Stream, Name: Durable, Config: nats.ConsumerConfig{Durable: Durable, FilterSubject: Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy}}
	}
	require.NoError(t, ValidateConsumer(fixture()))
	for _, mutate := range []func(*nats.ConsumerInfo){func(i *nats.ConsumerInfo) { i.Stream = "foreign" }, func(i *nats.ConsumerInfo) { i.Name = "foreign" }, func(i *nats.ConsumerInfo) { i.Config.Durable = "foreign" }, func(i *nats.ConsumerInfo) { i.Config.FilterSubject = "voice.>" }, func(i *nats.ConsumerInfo) { i.Config.FilterSubjects = []string{Subject} }, func(i *nats.ConsumerInfo) { i.Config.DeliverSubject = "push" }, func(i *nats.ConsumerInfo) { i.Config.DeliverGroup = "shared" }, func(i *nats.ConsumerInfo) { i.Config.DeliverPolicy = nats.DeliverNewPolicy }, func(i *nats.ConsumerInfo) { i.Config.AckPolicy = nats.AckNonePolicy }} {
		i := fixture()
		mutate(i)
		require.Error(t, ValidateConsumer(i))
	}
}

type blockedInbox struct {
	entered chan struct{}
	commit  chan struct{}
	calls   atomic.Int32
}

func (s *blockedInbox) ApplyVoiceRosterEvent(ctx context.Context, _ []byte) (store.VoiceRosterResetResult, error) {
	call := s.calls.Add(1)
	if call == 1 {
		close(s.entered)
		select {
		case <-s.commit:
		case <-ctx.Done():
			return store.VoiceRosterResetResult{}, ctx.Err()
		}
		return store.VoiceRosterResetResult{}, errors.New("transaction rolled back")
	}
	return store.VoiceRosterResetResult{ContractMismatch: true}, nil
}

func TestRosterPullConsumerAcksOnlyCommittedInboxAndRetriesFailedTransaction(t *testing.T) {
	if testing.Short() {
		t.Skip("real Linux NATS integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	broker, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{ContainerRequest: testcontainers.ContainerRequest{Image: "nats:2.11-alpine", Cmd: []string{"-js"}, ExposedPorts: []string{"4222/tcp"}, WaitingFor: wait.ForListeningPort("4222/tcp")}, Started: true})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, broker.Terminate(context.Background())) })
	host, err := broker.Host(ctx)
	require.NoError(t, err)
	port, err := broker.MappedPort(ctx, "4222/tcp")
	require.NoError(t, err)
	url := fmt.Sprintf("nats://%s", net.JoinHostPort(host, port.Port()))
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: Stream, Subjects: []string{Subject}, Storage: nats.FileStorage})
	require.NoError(t, err)
	_, err = js.AddConsumer(Stream, &nats.ConsumerConfig{Durable: Durable, FilterSubject: Subject, DeliverPolicy: nats.DeliverAllPolicy, AckPolicy: nats.AckExplicitPolicy, AckWait: 10 * time.Second})
	require.NoError(t, err)
	inbox := &blockedInbox{entered: make(chan struct{}), commit: make(chan struct{})}
	consumer, err := Start(url, inbox, nil)
	require.NoError(t, err)
	var alerts atomic.Int32
	consumer.OnContractMismatch = func() { alerts.Add(1) }
	done := make(chan error, 1)
	go func() { done <- consumer.Run(ctx) }()
	defer consumer.Close()
	_, err = js.Publish(Subject, []byte("immutable broker payload; inbox fixture owns interpretation"))
	require.NoError(t, err)
	select {
	case <-inbox.entered:
	case <-ctx.Done():
		t.Fatal("consumer did not enter transaction")
	}
	info, err := js.ConsumerInfo(Stream, Durable)
	require.NoError(t, err)
	require.EqualValues(t, 0, info.AckFloor.Stream)
	require.EqualValues(t, 1, info.NumAckPending)
	close(inbox.commit)
	require.Eventually(t, func() bool {
		info, err := js.ConsumerInfo(Stream, Durable)
		return err == nil && info.AckFloor.Stream == 1 && info.NumAckPending == 0
	}, 8*time.Second, 20*time.Millisecond)
	require.EqualValues(t, 2, inbox.calls.Load(), "failed transaction must be retried before ACK")
	require.EqualValues(t, 1, alerts.Load(), "durably handled conflict alerts before ACK")
	cancel()
	select {
	case err := <-done:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("consumer did not stop")
	}
}
