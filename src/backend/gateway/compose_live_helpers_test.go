package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type composeWSFrame struct {
	Op string          `json:"op"`
	S  int64           `json:"s"`
	D  json.RawMessage `json:"d"`
}

func composeWebSocketURL(t *testing.T, apiBase string) string {
	t.Helper()
	u, err := url.Parse(strings.TrimRight(apiBase, "/"))
	require.NoError(t, err)
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		t.Fatalf("unsupported API base scheme %q", u.Scheme)
	}
	u.Path = "/ws"
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func dialComposeRealtimeWS(t *testing.T, apiBase, accessToken string) *websocket.Conn {
	t.Helper()
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+accessToken)
	conn, resp, err := websocket.DefaultDialer.Dial(composeWebSocketURL(t, apiBase), hdr)
	if resp != nil {
		defer resp.Body.Close()
	}
	require.NoError(t, err, "WS /ws upgrade via gateway")
	require.NotNil(t, conn)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func waitComposeWSHello(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	frame := waitComposeWSOp(t, conn, "hello", 15*time.Second, nil)
	require.Greater(t, frame.S, int64(0))
}

func waitComposeWSOp(
	t *testing.T,
	conn *websocket.Conn,
	wantOp string,
	timeout time.Duration,
	match func(map[string]any) bool,
) composeWSFrame {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		_ = conn.SetReadDeadline(time.Now().Add(remaining))
		var frame composeWSFrame
		err := conn.ReadJSON(&frame)
		if err != nil {
			t.Fatalf("read WS while waiting for op=%s: %v", wantOp, err)
		}
		if frame.Op != wantOp {
			continue
		}
		if match == nil {
			return frame
		}
		var data map[string]any
		if len(frame.D) > 0 {
			require.NoError(t, json.Unmarshal(frame.D, &data))
		}
		if match(data) {
			return frame
		}
	}
	t.Fatalf("timeout waiting for WS op=%s", wantOp)
	return composeWSFrame{}
}

func createComposeDM(t *testing.T, client *http.Client, base, accessToken, otherProfileID string) string {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"other_profile_id": otherProfileID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/chats/dm", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /api/v1/chats/dm body=%s", string(body))

	var parsed struct {
		Chat struct {
			ID string `json:"id"`
		} `json:"chat"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Chat.ID)
	return parsed.Chat.ID
}

type composeCallSession struct {
	RoomID             string `json:"room_id"`
	LivekitRoomName    string `json:"livekit_room_name"`
	InitiatorProfileID string `json:"initiator_profile_id"`
	CalleeProfileID    string `json:"callee_profile_id"`
	MediaKind          string `json:"media_kind"`
	Status             string `json:"status"`
	LinkedChat         struct {
		ID string `json:"id"`
	} `json:"linked_chat"`
}

type composeJoinToken struct {
	JWT        string `json:"jwt"`
	LivekitURL string `json:"livekit_url"`
}

func startComposeCall(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, calleeProfileID string,
) composeCallSession {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"linked_chat":        map[string]string{"id": chatID},
		"callee_profile_id":  calleeProfileID,
		"media_kind":         "audio",
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/voice/calls", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST /api/v1/voice/calls body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.CallSession.RoomID)
	return parsed.CallSession
}

func acceptComposeCall(t *testing.T, client *http.Client, base, accessToken, roomID string) composeCallSession {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/accept", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST accept body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.CallSession
}

func getComposeJoinToken(t *testing.T, client *http.Client, base, accessToken, roomID string) composeJoinToken {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/token", base, roomID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET token body=%s", string(body))

	var parsed composeJoinToken
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.JWT)
	return parsed
}

func getComposeActiveCall(t *testing.T, client *http.Client, base, accessToken string) *composeCallSession {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/voice/calls/active", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET active body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	if parsed.CallSession.RoomID == "" {
		return nil
	}
	s := parsed.CallSession
	return &s
}

func endComposeCall(t *testing.T, client *http.Client, base, accessToken, roomID string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/end", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func liveLivekitURL() string {
	for _, key := range []string{"VOICE_LIVEKIT_URL", "VOICE_LIVEKIT_PUBLIC_URL"} {
		if u := strings.TrimSpace(os.Getenv(key)); u != "" {
			return u
		}
	}
	return "ws://127.0.0.1:7880"
}
