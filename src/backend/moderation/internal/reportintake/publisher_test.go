package reportintake

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	analyticsv1 "voice.app/voice/analytics/v1"
	eventsv1 "voice.app/voice/events/v1"
	"voice/backend/moderation/internal/store"
)

func TestReportIntakeRealJetStreamStableEvents(t *testing.T) {
	url := os.Getenv("REPORT_INTAKE_TEST_NATS_URL")
	if url == "" {
		t.Skip("requires an isolated disposable NATS JetStream instance")
	}
	nc, err := nats.Connect(url)
	require.NoError(t, err)
	defer nc.Close()
	js, err := nc.JetStream()
	require.NoError(t, err)
	// This opt-in endpoint is owned only by this fixture, never DEV/STAGING.
	for _, cfg := range []*nats.StreamConfig{{Name: "moderation_events", Subjects: []string{"moderation.report_created", "moderation.sanction_applied", "moderation.appeal_submitted"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour}, {Name: "analytics_events", Subjects: []string{"analytics.>"}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 7 * 24 * time.Hour}} {
		_ = js.DeleteStream(cfg.Name)
		_, err = js.AddStream(cfg)
		require.NoError(t, err)
		defer js.DeleteStream(cfg.Name)
	}
	publisher, err := NewPublisher(url)
	require.NoError(t, err)
	defer publisher.Close()
	claim := &store.ValidationClaim{ValidationFact: store.ValidationFact{ReportID: uuid.New(), ReporterProfileID: uuid.New(), TargetType: "story", TargetID: uuid.New(), CreatedAt: time.Now().UTC()}, Category: "spam"}
	events, err := createdEvents(claim)
	require.NoError(t, err)
	require.Len(t, events, 2)
	for _, event := range events {
		require.NoError(t, publisher.Publish(context.Background(), event))
		require.NoError(t, publisher.Publish(context.Background(), event))
		stream := "moderation_events"
		if event.Kind == "analytics.report_created" {
			stream = "analytics_events"
		}
		info, err := js.StreamInfo(stream)
		require.NoError(t, err)
		require.Equal(t, uint64(1), info.State.Msgs, "retry keeps one logical JetStream message inside dedup window")
		msg, err := js.GetMsg(stream, 1)
		require.NoError(t, err)
		require.Equal(t, event.Payload, msg.Data)
		require.Equal(t, event.ID.String(), msg.Header.Get(nats.MsgIdHdr))
		if stream == "moderation_events" {
			var decoded eventsv1.ModerationStreamEvent
			require.NoError(t, proto.Unmarshal(msg.Data, &decoded))
			require.Equal(t, event.ID.String(), decoded.EventId)
			require.Equal(t, claim.ReportID.String(), decoded.GetReportCreated().ReportId)
		} else {
			var decoded analyticsv1.AnalyticsEvent
			require.NoError(t, protojson.Unmarshal(msg.Data, &decoded))
			require.Equal(t, event.ID.String(), decoded.EventId)
			require.JSONEq(t, `{"target_type":"story","category":"spam"}`, decoded.PropertiesJson)
		}
	}
	altered := events[0]
	altered.Subject = "moderation.sanction_applied"
	require.Error(t, publisher.Publish(context.Background(), altered))
	require.NoError(t, js.DeleteStream("analytics_events"))
	require.Error(t, publisher.Publish(context.Background(), events[1]), "missing deployment stream remains retryable error")
}
