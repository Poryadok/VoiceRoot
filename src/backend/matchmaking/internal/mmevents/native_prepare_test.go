package mmevents

import (
	"context"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
	"time"
)

func TestActualNativeBrokerPreparationChecksExistingCanonicalStreamOnly(t *testing.T) {
	url := os.Getenv("BE011_TEST_NATS_URL")
	if url == "" {
		t.Skip("owned NATS unavailable")
	}
	admin, err := nats.Connect(url)
	require.NoError(t, err)
	defer admin.Close()
	t.Logf("actual NATS %s; explicit broker fixture, not installed credentials", admin.ConnectedServerVersion())
	js, err := admin.JetStream()
	require.NoError(t, err)
	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	defer pub.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.Error(t, pub.CheckNativeConnection(ctx))
	_, err = js.StreamInfo(streamName)
	require.ErrorIs(t, err, nats.ErrStreamNotFound, "preparation must not create a stream")
	config := &nats.StreamConfig{Name: streamName, Subjects: []string{subjectSearchStarted}, Storage: nats.FileStorage, Retention: nats.LimitsPolicy, MaxAge: 168 * time.Hour}
	_, err = js.AddStream(config)
	require.NoError(t, err)
	defer js.DeleteStream(streamName)
	require.Error(t, pub.CheckNativeConnection(ctx), "one valid subject cannot qualify an incomplete publisher contract")
	config.Subjects = []string{subjectSearchStarted, subjectSearchCancel, subjectSearchNudge, subjectSearchTimeout, subjectMatchFound, subjectMatchCompleted, subjectRatingSubmitted, subjectPlayerBanned}
	_, err = js.UpdateStream(config)
	require.NoError(t, err)
	require.NoError(t, pub.CheckNativeConnection(ctx))
	info, err := js.StreamInfo(streamName)
	require.NoError(t, err)
	require.Zero(t, info.State.Msgs, "readiness must not publish probe domain events")
	config.Storage = nats.MemoryStorage
	// Storage cannot be changed in-place. Recreate this own empty fixture only.
	require.NoError(t, js.DeleteStream(streamName))
	_, err = js.AddStream(config)
	require.NoError(t, err)
	require.Error(t, pub.CheckNativeConnection(ctx), "memory storage is not the canonical retained stream")
}
