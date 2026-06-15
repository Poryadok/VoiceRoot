package webhook_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"voice/backend/bot/internal/webhook"
)

func TestDeliverPOST_retriesOn5xx(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":"ok"}`))
	}))
	defer srv.Close()

	resp, err := webhook.DeliverPOST(context.Background(), nil, srv.URL, "secret", webhook.InteractionPayload{
		Type:             "slash_command",
		InteractionToken: "tok",
		CommandName:      "ping",
	}, time.Second)
	require.NoError(t, err)
	require.Equal(t, "ok", resp.Content)
	require.GreaterOrEqual(t, hits.Load(), int32(3))
}
