package analyticsevents

import (
	"context"
	"errors"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"

	analyticsv1 "voice.app/voice/analytics/v1"
	"voice/backend/pkg/analyticshash"
)

type capturedAnalyticsJetStream struct {
	nats.JetStreamContext
	message *nats.Msg
	err     error
}

func (s *capturedAnalyticsJetStream) PublishMsg(message *nats.Msg, _ ...nats.PubOpt) (*nats.PubAck, error) {
	s.message = message
	if s.err != nil {
		return nil, s.err
	}
	return &nats.PubAck{Stream: streamName, Sequence: 1}, nil
}

func TestAnalyticsPublisherProducesBinaryEnvelope(t *testing.T) {
	js := &capturedAnalyticsJetStream{}
	publisher := &JetStreamPublisher{js: js, HashKey: "test-hash-key"}
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("x-request-id", "request-369"))
	accountID := "11111111-1111-1111-1111-111111111111"
	require.NoError(t, publisher.PublishWithAccount(ctx, "analytics.gateway.request", "gateway", "request", accountID, map[string]any{"route": "/api/v1/chats"}))
	require.NotNil(t, js.message)
	require.Equal(t, "analytics.gateway.request", js.message.Subject)
	require.Equal(t, "request-369", js.message.Header.Get("X-Request-Id"))
	var event analyticsv1.AnalyticsEvent
	require.NoError(t, proto.Unmarshal(js.message.Data, &event), "telemetry must use protobuf bytes rather than ProtoJSON")
	require.NotEmpty(t, event.GetEventId())
	require.NoError(t, event.GetTimestamp().CheckValid())
	require.Equal(t, "gateway", event.GetSourceService())
	require.Equal(t, "request", event.GetEventType())
	require.JSONEq(t, `{"route":"/api/v1/chats"}`, event.GetPropertiesJson())
	require.Equal(t, analyticshash.ID("test-hash-key", accountID), event.GetUserIdHashed())
	require.NotContains(t, string(js.message.Data), accountID)
}

func TestAnalyticsBinaryPublisherPreservesPublishFailure(t *testing.T) {
	want := errors.New("JetStream unavailable")
	js := &capturedAnalyticsJetStream{err: want}
	publisher := &JetStreamPublisher{js: js}
	require.ErrorIs(t, publisher.Publish(context.Background(), "analytics.gateway.request", "gateway", "request", nil), want)
	var event analyticsv1.AnalyticsEvent
	require.NoError(t, proto.Unmarshal(js.message.Data, &event))
	require.Equal(t, "{}", event.GetPropertiesJson())
}
