package entitlementoutbox

import (
	"context"
	"crypto/sha256"
	"github.com/google/uuid"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func jetstream(t *testing.T) nats.JetStreamContext {
	t.Helper()
	srv, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go srv.Start()
	require.True(t, srv.ReadyForConnections(5*time.Second))
	t.Cleanup(srv.Shutdown)
	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "subscription_events", Subjects: []string{Subject}, Storage: nats.MemoryStorage})
	require.NoError(t, err)
	return js
}

func TestPublishStoredBytesStableJetStreamDedup(t *testing.T) {
	js := jetstream(t)
	ev := snapshotEvent()
	body, err := proto.MarshalOptions{Deterministic: true}.Marshal(ev)
	require.NoError(t, err)
	hash := sha256.Sum256(body)
	row := Delivery{EventID: ev.EventId, Payload: body, PayloadHash: hash[:]}
	first, err := Publish(context.Background(), js, row)
	require.NoError(t, err)
	second, err := Publish(context.Background(), js, row) // crash after PubAck before completion
	require.NoError(t, err)
	require.Equal(t, first, second)
	info, err := js.StreamInfo("subscription_events")
	require.NoError(t, err)
	require.EqualValues(t, 1, info.State.Msgs)
	stored, err := js.GetMsg(first.Stream, first.Sequence)
	require.NoError(t, err)
	require.Equal(t, body, stored.Data)
	require.Equal(t, ev.EventId, stored.Header.Get(nats.MsgIdHdr))
	row.EventID = uuid.NewString()
	_, err = Publish(context.Background(), js, row)
	require.ErrorIs(t, err, ErrContractMismatch)
	ev.EventId = row.EventID
	row.Payload, err = proto.MarshalOptions{Deterministic: true}.Marshal(ev)
	require.NoError(t, err)
	corruptHash := sha256.Sum256(row.Payload)
	row.PayloadHash = corruptHash[:]
	row.PayloadHash[0] ^= 1
	_, err = Publish(context.Background(), js, row)
	require.ErrorIs(t, err, ErrContractMismatch)
	info, err = js.StreamInfo("subscription_events")
	require.NoError(t, err)
	require.EqualValues(t, 1, info.State.Msgs)
}

func TestCommittedClaimPublishesAfterRestart(t *testing.T) {
	pool := database(t)
	js := jetstream(t)
	ctx := context.Background()
	ev := snapshotEvent()
	commitEvent(t, pool, 0, ev)
	// A fresh dispatcher sees the transaction committed by a dead producer.
	s := Store{Pool: pool}
	rows, err := s.Claim(ctx, 1, time.Minute)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	ack, err := Publish(ctx, js, rows[0])
	require.NoError(t, err)
	stored, err := js.GetMsg(ack.Stream, ack.Sequence)
	require.NoError(t, err)
	want, err := proto.MarshalOptions{Deterministic: true}.Marshal(ev)
	require.NoError(t, err)
	require.Equal(t, want, stored.Data)
	require.Equal(t, ev.EventId, stored.Header.Get(nats.MsgIdHdr))
	require.NoError(t, s.Complete(ctx, rows[0], ack))
}
