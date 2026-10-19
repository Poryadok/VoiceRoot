package reportthreshold

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"voice/backend/moderation/internal/store"
)

func TestPublisherRetainsDurableEventIDAndBytes(t *testing.T) {
	ns, err := server.NewServer(&server.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true})
	require.NoError(t, err)
	go ns.Start()
	require.True(t, ns.ReadyForConnections(5*time.Second))
	t.Cleanup(func() { ns.Shutdown(); ns.WaitForShutdown() })
	nc, err := nats.Connect(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{Name: "moderation_events", Subjects: []string{"moderation.report_created", "moderation.sanction_applied", "moderation.appeal_submitted"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour})
	require.NoError(t, err)
	publisher, err := NewPublisher(ns.ClientURL())
	require.NoError(t, err)
	t.Cleanup(publisher.Close)
	events, err := Events(store.ReportThresholdCase{ID: uuid.New(), SanctionID: uuid.New(), AccountID: uuid.New(), ProfileID: uuid.New(), CreatedAt: time.Now().UTC()})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.NoError(t, publisher.Publish(context.Background(), events[0]))
	require.NoError(t, publisher.Publish(context.Background(), events[0]))
	info, err := js.StreamInfo("moderation_events")
	require.NoError(t, err)
	require.Equal(t, uint64(1), info.State.Msgs)
	message, err := js.GetMsg("moderation_events", 1)
	require.NoError(t, err)
	require.Equal(t, events[0].Payload, message.Data)
	require.Equal(t, events[0].ID.String(), message.Header.Get(nats.MsgIdHdr))
	tampered := events[0]
	tampered.ID = uuid.New()
	require.Error(t, publisher.Publish(context.Background(), tampered))
	tampered = events[0]
	tampered.Subject = "analytics.other"
	require.Error(t, publisher.Publish(context.Background(), tampered))
}
