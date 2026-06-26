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

func TestChatGRPC_logPublishError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	publishErr := errors.New("jetstream publish chat.created: nats: timeout")

	s := &ChatGRPC{Logger: logger}
	s.logPublishError(context.Background(), "chat.created", publishErr)

	out := buf.String()
	require.Contains(t, out, "nats_publish")
	require.Contains(t, out, "chat.created")
	require.Contains(t, out, publishErr.Error())

	var rec map[string]any
	require.NoError(t, json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &rec))
	require.Equal(t, "nats_publish", rec["event"])
	require.Equal(t, "chat.created", rec["subject"])
	require.Equal(t, publishErr.Error(), rec["error"])
}
