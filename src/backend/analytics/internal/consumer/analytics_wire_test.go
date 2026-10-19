package consumer

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/analytics/internal/adapters"
	"voice/backend/analytics/internal/buffer"
	"voice/backend/analytics/internal/store"
	"voice/backend/pkg/analyticsevents"
)

func TestHandleAnalyticsMsgPreservesBinaryAndRetainedJSON(t *testing.T) {
	accountHash := "already-hashed-account"
	event := &analyticsv1.AnalyticsEvent{
		EventId: "11111111-1111-4111-8111-111111111111", EventType: "request",
		SourceService: "gateway", Timestamp: timestamppb.New(time.Unix(1700000000, 0)),
		UserIdHashed: &accountHash, PropertiesJson: `{"route":"/api/v1/chats"}`,
	}
	for _, encoding := range []struct {
		name    string
		marshal func(proto.Message) ([]byte, error)
	}{{"binary", proto.Marshal}, {"retained ProtoJSON", protojson.Marshal}} {
		t.Run(encoding.name, func(t *testing.T) {
			data, err := encoding.marshal(event)
			require.NoError(t, err)
			var rows []store.EventRow
			accumulator := buffer.New(1, time.Hour, func(_ context.Context, batch []store.EventRow) error {
				rows = append(rows, batch...)
				return nil
			}, nil)
			runner := &Runner{Mapper: adapters.Mapper{}, Buffer: accumulator}
			message := &nats.Msg{Subject: "analytics.gateway.request", Data: data}
			require.NoError(t, runner.handleAnalyticsMsg(message))
			// Redelivery keeps the exact logical identity in either encoding.
			require.NoError(t, runner.handleAnalyticsMsg(message))
			require.Len(t, rows, 2)
			for _, row := range rows {
				require.Equal(t, event.GetEventId(), row.EventID)
				require.Equal(t, event.GetEventType(), row.EventType)
				require.Equal(t, event.GetSourceService(), row.SourceService)
				require.Equal(t, event.GetTimestamp().AsTime(), row.Timestamp)
				require.Equal(t, accountHash, row.UserIDHashed)
				require.JSONEq(t, event.GetPropertiesJson(), row.PropertiesJSON)
			}
		})
	}
}

func TestAnalyticsPublisherJetStreamBinaryToConsumer(t *testing.T) {
	server := startAnalyticsJetStream(t)
	connection, err := nats.Connect(server.ClientURL())
	require.NoError(t, err)
	t.Cleanup(connection.Close)
	js, err := connection.JetStream()
	require.NoError(t, err)
	_, err = js.AddStream(&nats.StreamConfig{
		Name: "analytics_events", Subjects: []string{"analytics.>"},
		Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour, Storage: nats.FileStorage,
	})
	require.NoError(t, err)
	const durable = "analytics_wire"
	provisionAnalyticsDurable(t, js, "analytics_events", "analytics.>", durable, durable)
	subscription, err := js.QueueSubscribeSync("analytics.>", durable, nats.Bind("analytics_events", durable), nats.ManualAck())
	require.NoError(t, err)
	t.Cleanup(func() { _ = subscription.Unsubscribe() })
	publisher, err := analyticsevents.NewJetStreamPublisher(server.ClientURL(), "gateway")
	require.NoError(t, err)
	t.Cleanup(publisher.Close)
	publisher.HashKey = "test-hash-key"
	require.NoError(t, publisher.PublishWithAccount(context.Background(), "analytics.gateway.request", "gateway", "request", "11111111-1111-1111-1111-111111111111", map[string]any{"route": "/api/v1/chats"}))
	message, err := subscription.NextMsg(time.Second)
	require.NoError(t, err)
	var sent analyticsv1.AnalyticsEvent
	require.NoError(t, proto.Unmarshal(message.Data, &sent))
	var rows []store.EventRow
	accumulator := buffer.New(1, time.Hour, func(_ context.Context, batch []store.EventRow) error {
		rows = append(rows, batch...)
		return nil
	}, nil)
	runner := &Runner{Mapper: adapters.Mapper{}, Buffer: accumulator}
	require.NoError(t, runner.handleAnalyticsMsg(message))
	require.Equal(t, []store.EventRow{store.RowFromProto(&sent)}, rows)
	require.Eventually(t, func() bool {
		info, infoErr := js.ConsumerInfo("analytics_events", durable)
		return infoErr == nil && info.NumAckPending == 0 && info.AckFloor.Stream == 1
	}, time.Second, 10*time.Millisecond, "successful persistence must acknowledge the source message")
}
