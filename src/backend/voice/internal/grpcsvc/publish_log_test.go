package grpcsvc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVoiceGRPC_logPublishError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	publishErr := errors.New("jetstream publish voice.call_incoming: nats: timeout")

	s := &VoiceGRPC{Logger: logger}
	s.logPublishError(context.Background(), "voice.call_incoming", publishErr)

	out := buf.String()
	require.Contains(t, out, "nats_publish")
	require.Contains(t, out, "voice.call_incoming")
	require.Contains(t, out, publishErr.Error())

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	require.Equal(t, "nats_publish", rec["event"])
	require.Equal(t, "voice.call_incoming", rec["subject"])
	require.Equal(t, publishErr.Error(), rec["error"])
}
