package subscriptionconsume

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	eventsv1 "voice.app/voice/events/v1"
)

type retryEntitlements struct{ calls atomic.Int32 }

func (*retryEntitlements) UpsertSpaceSubscription(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (r *retryEntitlements) FinalizeSpacePro(context.Context, uuid.UUID) error {
	if r.calls.Add(1) == 1 {
		return errors.New("transient store failure")
	}
	return nil
}

func TestSpaceEntitlementDurableBindAndRedelivery(t *testing.T) {
	if testing.Short() {
		t.Skip("embedded NATS integration")
	}
	s, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, NoLog: true, NoSigs: true, JetStream: true, StoreDir: t.TempDir()})
	require.NoError(t, err)
	go s.Start()
	require.True(t, s.ReadyForConnections(5*time.Second))
	t.Cleanup(s.Shutdown)
	nc, err := nats.Connect(s.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: streamName, Subjects: []string{"subscription.>"}})
	require.NoError(t, err)
	store := &retryEntitlements{}
	_, err = Start(context.Background(), s.ClientURL(), defaultDurable, store)
	require.Error(t, err, "missing durable must not be created by the application")
	_, err = js.ConsumerInfo(streamName, defaultDurable)
	require.Error(t, err)
	config := &nats.ConsumerConfig{Durable: defaultDurable, FilterSubjects: []string{subjectSpaceProStarted, subjectSpaceProExpired}, DeliverSubject: spaceDeliverySubject, DeliverPolicy: nats.DeliverNewPolicy, AckPolicy: nats.AckExplicitPolicy}
	_, err = js.AddConsumer(streamName, config)
	require.NoError(t, err)
	c, err := Start(context.Background(), s.ClientURL(), defaultDurable, store)
	require.NoError(t, err)
	defer c.Close()
	raw, err := proto.Marshal(&eventsv1.SubscriptionStreamEvent{Payload: &eventsv1.SubscriptionStreamEvent_SpaceProExpired{SpaceProExpired: &eventsv1.SpaceProExpired{SpaceId: uuid.NewString()}}})
	require.NoError(t, err)
	_, err = js.Publish(subjectSpaceProExpired, raw)
	require.NoError(t, err)
	require.Eventually(t, func() bool { return store.calls.Load() >= 2 }, 5*time.Second, 10*time.Millisecond, "transient failure must NAK and redeliver")
	require.Eventually(t, func() bool {
		info, err := js.ConsumerInfo(streamName, defaultDurable)
		return err == nil && info.AckFloor.Stream == 1 && info.NumAckPending == 0
	}, 5*time.Second, 10*time.Millisecond, "successful retry must ACK")
	c.Close()
	require.NoError(t, js.DeleteConsumer(streamName, defaultDurable))
	config.FilterSubjects = []string{subjectSpaceProStarted, "subscription.user_premium_started"}
	_, err = js.AddConsumer(streamName, config)
	require.NoError(t, err)
	_, err = Start(context.Background(), s.ClientURL(), defaultDurable, store)
	require.Error(t, err, "drifted neighboring subject must fail closed")
}
