package fileevents

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

func TestJetStreamPublisher_FileUploadedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectFileUploaded)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const fileID, profileID = "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	require.NoError(t, pub.PublishFileUploaded(ctx, fileID, profileID))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.FileStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	require.NotEmpty(t, env.GetEventId())
	uploaded := env.GetFileUploaded()
	require.NotNil(t, uploaded)
	require.Equal(t, fileID, uploaded.GetFileId())
	require.Equal(t, profileID, uploaded.GetUploaderProfileId())
}

func TestJetStreamPublisher_FileScanInfectedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectFileScanInfected)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const fileID, profileID = "33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"
	require.NoError(t, pub.PublishFileScanInfected(ctx, fileID, profileID))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.FileStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	scan := env.GetFileScanResult()
	require.NotNil(t, scan)
	require.Equal(t, fileID, scan.GetFileId())
	require.Equal(t, "infected", scan.GetResult())
	require.Equal(t, profileID, scan.GetUploaderProfileId())
}

func TestJetStreamPublisher_FileProcessedRoundTrip(t *testing.T) {
	ctx := context.Background()
	s := startJSTestServer(t)
	url := s.ClientURL()

	nc, err := nats.Connect(url)
	require.NoError(t, err)
	t.Cleanup(nc.Close)

	sub, err := nc.SubscribeSync(subjectFileProcessed)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sub.Unsubscribe() })

	pub, err := NewJetStreamPublisher(url)
	require.NoError(t, err)
	t.Cleanup(func() { _ = pub.Close() })

	const fileID = "55555555-5555-4555-8555-555555555555"
	converted := "processed/" + fileID + "/full.webp"
	thumb := "processed/" + fileID + "/thumb.webp"
	require.NoError(t, pub.PublishFileProcessed(ctx, fileID, "ready", converted, thumb))

	msg, err := sub.NextMsg(3 * time.Second)
	require.NoError(t, err)
	var env eventsv1.FileStreamEvent
	require.NoError(t, proto.Unmarshal(msg.Data, &env))
	processed := env.GetFileProcessed()
	require.NotNil(t, processed)
	require.Equal(t, fileID, processed.GetFileId())
	require.Equal(t, "ready", processed.GetStatus())
	require.Equal(t, converted, processed.GetConvertedR2Key())
	require.Equal(t, thumb, processed.GetThumbnailR2Key())
}
