package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

// TestComposeRealtimeGateway_live exercises core REST (chats, friends, users/search)
// and /ws → Realtime through Gateway (docker compose --profile app).
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeRealtimeGateway_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 30 * time.Second}
	base := liveGatewayBaseURL()

	healthResp, err := client.Get(base + "/health")
	require.NoError(t, err)
	healthResp.Body.Close()
	require.Equal(t, http.StatusOK, healthResp.StatusCode, "gateway health at %s", base)

	n := time.Now().UnixNano()
	emailA := fmt.Sprintf("rt-gw-a-%d@voice-qa.test", n)
	emailB := fmt.Sprintf("rt-gw-b-%d@voice-qa.test", n)
	const password = "VoiceQaTest1!"

	sessA := registerComposeUser(t, client, base, emailA, password)
	registerComposeUser(t, client, base, emailB, password)

	auth := map[string]string{"Authorization": "Bearer " + sessA.AccessToken}
	for _, spec := range []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/chats"},
		{http.MethodGet, "/api/v1/friends"},
	} {
		req, err := http.NewRequest(spec.method, base+spec.path, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", auth["Authorization"])
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode,
			"%s %s must not 404 once gRPC upstreams are wired; body=%s", spec.method, spec.path, string(body))
	}

	searchToken := strings.TrimSuffix(emailB, "@voice-qa.test")
	searchURL := base + "/api/v1/users/search?q=" + searchToken
	searchReq, err := http.NewRequest(http.MethodGet, searchURL, nil)
	require.NoError(t, err)
	searchReq.Header.Set("Authorization", auth["Authorization"])
	searchResp, err := client.Do(searchReq)
	require.NoError(t, err)
	defer searchResp.Body.Close()
	searchBody, _ := io.ReadAll(searchResp.Body)
	require.Equal(t, http.StatusOK, searchResp.StatusCode,
		"GET /api/v1/users/search must not 404 once users upstream is wired; body=%s", string(searchBody))

	wsBase, err := url.Parse(base)
	require.NoError(t, err)
	switch wsBase.Scheme {
	case "http":
		wsBase.Scheme = "ws"
	case "https":
		wsBase.Scheme = "wss"
	default:
		t.Fatalf("unsupported API base scheme %q", wsBase.Scheme)
	}
	wsBase.Path = "/ws"
	wsBase.RawQuery = ""
	wsBase.Fragment = ""

	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+sessA.AccessToken)
	conn, resp, err := websocket.DefaultDialer.Dial(wsBase.String(), hdr)
	if resp != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err, "WS /ws must upgrade via GATEWAY_REALTIME_UPSTREAM_URL, not 404")
	require.NotNil(t, conn)
	t.Cleanup(func() { _ = conn.Close() })

	_ = conn.SetReadDeadline(time.Now().Add(15 * time.Second))
	var frame struct {
		Op string `json:"op"`
		S  int64  `json:"s"`
	}
	require.NoError(t, conn.ReadJSON(&frame))
	require.Equal(t, "hello", frame.Op, "realtime must send hello after gateway-proxied upgrade")
	require.Greater(t, frame.S, int64(0))
}

// TestComposeRealtimeGateway_wsWithoutUpstream_404 documents that /ws returns 404 when
// GATEWAY_REALTIME_UPSTREAM_URL is unset (in-process contract).
func TestComposeRealtimeGateway_wsWithoutUpstream_404(t *testing.T) {
	t.Parallel()

	h := newGatewayForContract(t, gatewayTestOptions{
		tokenClaims: map[string]tokenClaims{
			"jwt": {UserID: "acc-1", ProfileID: "prof-1"},
		},
	})

	rec := performRequest(h, http.MethodGet, "/ws", "", map[string]string{
		"Authorization":         "Bearer jwt",
		"Connection":            "Upgrade",
		"Upgrade":               "websocket",
		"Sec-WebSocket-Key":     "dGhlIHNhbXBsZSBub25jZQ==",
		"Sec-WebSocket-Version": "13",
	})
	require.Equal(t, http.StatusNotFound, rec.Code)
}
