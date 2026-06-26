package natslog

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"

	"voice/backend/pkg/correlation"
)

func TestRequestIDFromMsg(t *testing.T) {
	require.Equal(t, "", RequestIDFromMsg(nil))
	require.Equal(t, "", RequestIDFromMsg(&nats.Msg{}))
	msg := &nats.Msg{Header: nats.Header{}}
	SetRequestIDHeader(msg.Header, "rid-1")
	require.Equal(t, "rid-1", RequestIDFromMsg(msg))
}

func TestSetRequestIDHeader_NoOp(t *testing.T) {
	hdr := nats.Header{}
	SetRequestIDHeader(hdr, "")
	SetRequestIDHeader(nil, "x")
	require.Empty(t, hdr.Get(correlation.RequestIDHeader))
}

func TestLogPublishAndConsume(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	LogPublish(logger, "chat.events", "req-1", "published")
	require.Contains(t, buf.String(), "nats_publish")
	require.Contains(t, buf.String(), "req-1")

	buf.Reset()
	msg := &nats.Msg{Subject: "chat.events", Header: nats.Header{}}
	SetRequestIDHeader(msg.Header, "req-2")
	LogConsume(logger, msg, slog.LevelWarn, "consumed")
	require.Contains(t, buf.String(), "nats_consume")
	require.Contains(t, buf.String(), "req-2")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	require.Equal(t, "nats_consume", rec["event"])
}

func TestLogPublishError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))

	LogPublishError(logger, "message.sent", "req-err", errors.New("jetstream unavailable"))
	require.Contains(t, buf.String(), "nats_publish")
	require.Contains(t, buf.String(), "req-err")
	require.Contains(t, buf.String(), "jetstream unavailable")

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	require.Equal(t, "nats_publish", rec["event"])
	require.Equal(t, "message.sent", rec["subject"])
	require.Equal(t, "req-err", rec["request_id"])
	require.Equal(t, "jetstream unavailable", rec["error"])
}

