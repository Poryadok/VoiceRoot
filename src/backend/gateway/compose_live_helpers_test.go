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

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func formatComposeEmail(prefix string, n int64) string {
	return fmt.Sprintf("%s-%d@voice-qa.test", prefix, n)
}

func composeClientMessageID() string {
	return uuid.NewString()
}

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

func composeWSSend(t *testing.T, conn *websocket.Conn, payload map[string]any) {
	t.Helper()
	require.NoError(t, conn.WriteJSON(payload))
}

func connectComposeWSSubscribed(t *testing.T, base, accessToken, chatID string) *websocket.Conn {
	t.Helper()
	conn := dialComposeRealtimeWS(t, base, accessToken)
	waitComposeWSHello(t, conn)
	composeWSSend(t, conn, map[string]any{
		"op": "subscribe",
		"d":  map[string]string{"chat_id": chatID},
	})
	waitComposeWSOp(t, conn, "subscribe_ack", 15*time.Second, func(d map[string]any) bool {
		return d["chat_id"] == chatID
	})
	return conn
}

func sendComposeMessage(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, content string,
) string {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":              map[string]string{"id": chatID},
		"content":           content,
		"client_message_id": composeClientMessageID(),
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/send", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST send body=%s", string(body))

	var parsed struct {
		Message struct {
			ID string `json:"id"`
		} `json:"message"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	require.NotEmpty(t, parsed.Message.ID)
	return parsed.Message.ID
}

func getComposeMessagesContains(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID, messageID, content string,
) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages?chat_id="+chatID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET messages body=%s", string(body))

	var hist struct {
		MessageList struct {
			Messages []struct {
				ID      string `json:"id"`
				Content string `json:"content"`
			} `json:"messages"`
		} `json:"message_list"`
	}
	require.NoError(t, json.Unmarshal(body, &hist))
	for _, m := range hist.MessageList.Messages {
		if m.ID == messageID && m.Content == content {
			return
		}
	}
	t.Fatalf("message %s with content %q not in history: %s", messageID, content, string(body))
}

func refreshComposeSession(t *testing.T, client *http.Client, base, refreshToken string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"refresh_token":    refreshToken,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	resp, err := client.Post(base+"/api/v1/auth/refresh", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "refresh body=%s", string(raw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	sess := envelope.Session
	require.NotEmpty(t, sess.AccessToken)
	require.NotEmpty(t, sess.RefreshToken)
	return sess
}

func logoutComposeSession(t *testing.T, client *http.Client, base, accessToken, refreshToken string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"refresh_token": refreshToken})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/logout", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func composeProtectedRouteStatus(t *testing.T, client *http.Client, base, accessToken string) int {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/chats", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func markReadComposeMessage(t *testing.T, client *http.Client, base, accessToken, chatID, messageID string) {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"chat":                  map[string]string{"id": chatID},
		"last_read_message_id":  messageID,
	})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/messages/read", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "mark read body=%s", string(body))
}

func getComposeReadState(t *testing.T, client *http.Client, base, accessToken, chatID string) string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/messages/read-state?chat_id="+chatID, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "read-state body=%s", string(body))

	var parsed struct {
		ReadState struct {
			LastReadMessageID string `json:"last_read_message_id"`
			ProfileID         string `json:"profile_id"`
			Chat              struct {
				ID string `json:"id"`
			} `json:"chat"`
		} `json:"read_state"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.ReadState.LastReadMessageID
}

func sendComposeFriendInvitation(t *testing.T, client *http.Client, base, accessToken, targetProfileID string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"target_profile_id": targetProfileID})
	require.NoError(t, err)
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/friends/invitations", bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "friend invitation body=%s", string(body))
}

func acceptComposeFriendInvitation(t *testing.T, client *http.Client, base, accessToken, requesterProfileID string) {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/friends/invitations/%s/accept", base, requesterProfileID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "accept friend body=%s", string(body))
}

func composeFriendIDs(t *testing.T, client *http.Client, base, accessToken string) []string {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/api/v1/friends", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "list friends body=%s", string(body))

	var parsed struct {
		FriendList struct {
			Friends []struct {
				ProfileID string `json:"profile_id"`
			} `json:"friends"`
		} `json:"friend_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	var ids []string
	for _, f := range parsed.FriendList.Friends {
		ids = append(ids, f.ProfileID)
	}
	return ids
}

func declineComposeCall(t *testing.T, client *http.Client, base, accessToken, roomID string) composeCallSession {
	t.Helper()
	url := fmt.Sprintf("%s/api/v1/voice/calls/%s/decline", base, roomID)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "POST decline body=%s", string(body))

	var parsed struct {
		CallSession composeCallSession `json:"call_session"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	return parsed.CallSession
}

func liveLivekitURL() string {
	for _, key := range []string{"VOICE_LIVEKIT_URL", "VOICE_LIVEKIT_PUBLIC_URL"} {
		if u := strings.TrimSpace(os.Getenv(key)); u != "" {
			return u
		}
	}
	return "ws://127.0.0.1:7880"
}
