package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestComposeTChatBlockedDMTransportClosure_live is the executable transport
// closure slice of the T-CHAT multi-client acceptance contract. The runner
// executes prepare, an active-socket lost-event phase, then restart verification.
//
// The test deliberately uses only public Gateway REST and WebSocket surfaces.
// It proves that a durable Social block which already denies both REST send
// directions cannot be bypassed by a fresh Realtime bootstrap or lazy WS
// subscribe after the service restart.
func TestComposeTChatBlockedDMTransportClosure_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}

	phase := strings.TrimSpace(os.Getenv("VOICE_TCHAT_CLOSURE_PHASE"))
	statePath := strings.TrimSpace(os.Getenv("VOICE_TCHAT_CLOSURE_STATE_PATH"))
	proofID := strings.TrimSpace(os.Getenv("VOICE_TCHAT_CLOSURE_PROOF_ID"))
	runStarted := strings.TrimSpace(os.Getenv("VOICE_TCHAT_CLOSURE_RUN_STARTED_UNIX_NANO"))
	if phase == "" || statePath == "" {
		t.Skip("T-CHAT closure proof requires external runner phase and state path")
	}
	require.Contains(t, []string{"prepare", "active", "verify"}, phase)
	require.True(t, filepath.IsAbs(statePath), "state path must be absolute")
	require.NotEmpty(t, proofID, "runner proof id is required")
	startedAt, err := strconv.ParseInt(runStarted, 10, 64)
	require.NoError(t, err, "runner start time must be Unix nanoseconds")

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	if phase == "prepare" {
		prepareTChatBlockedDMTransportClosure(t, client, base, statePath, proofID, startedAt)
		return
	}
	if phase == "active" {
		verifyTChatActiveSocketClosureWithoutEvent(t, client, base, statePath, proofID, startedAt)
		return
	}
	verifyTChatBlockedDMTransportClosure(t, client, base, statePath, proofID, startedAt)
}

type tchatClosureState struct {
	ProofID        string   `json:"proof_id"`
	CreatedAt      int64    `json:"created_at_unix_nano"`
	ChatID         string   `json:"chat_id"`
	AccountA       string   `json:"account_a"`
	AccountB       string   `json:"account_b"`
	ProfileA       string   `json:"profile_a"`
	ProfileB       string   `json:"profile_b"`
	AccessTokenA   string   `json:"access_token_a"`
	AccessTokenB   string   `json:"access_token_b"`
	HistoryMessage []string `json:"history_message_ids"`
}

func prepareTChatBlockedDMTransportClosure(
	t *testing.T,
	client *http.Client,
	base, statePath, proofID string,
	runStarted int64,
) {
	t.Helper()
	clearLiveComposeAuthRateLimit(t)

	n := time.Now().UnixNano()
	const password = "VoiceQaTest1!"
	emailA := formatComposeEmail("tchat-closure-a", n)
	emailB := formatComposeEmail("tchat-closure-b", n)
	registeredA := registerComposeUser(t, client, base, emailA, password)
	registeredB := registerComposeUser(t, client, base, emailB, password)
	a := loginComposeUser(t, client, base, emailA, password)
	b := loginComposeUser(t, client, base, emailB, password)
	require.Equal(t, registeredA.AccountID, a.AccountID)
	require.Equal(t, registeredB.AccountID, b.AccountID)
	require.Equal(t, registeredA.ProfileID, a.ProfileID)
	require.Equal(t, registeredB.ProfileID, b.ProfileID)
	require.NotEqual(t, a.AccountID, b.AccountID)
	require.NotEqual(t, a.ProfileID, b.ProfileID)

	setComposePrivacyAllowDmEveryone(t, client, base, a.AccessToken)
	setComposePrivacyAllowDmEveryone(t, client, base, b.AccessToken)
	chatID := createComposeDM(t, client, base, a.AccessToken, b.ProfileID)
	requestMessageID := sendComposeMessage(t, client, base, a.AccessToken, chatID, "tchat-message-request")
	require.Nil(t, composeA1ChatItem(listComposeChats(t, client, base, b.AccessToken, "main"), chatID))
	requestItem := composeA1ChatItem(listComposeChats(t, client, base, b.AccessToken, "requests"), chatID)
	require.NotNil(t, requestItem, "first stranger message must enter requests")
	require.True(t, requestItem.IsStranger)

	acceptComposeDMRequest(t, client, base, b.AccessToken, chatID)
	sendComposeFriendInvitation(t, client, base, a.AccessToken, b.ProfileID)
	acceptComposeFriendInvitation(t, client, base, b.AccessToken, a.ProfileID)
	require.Contains(t, composeFriendIDs(t, client, base, a.AccessToken), b.ProfileID)
	require.Contains(t, composeFriendIDs(t, client, base, b.AccessToken), a.ProfileID)

	secondMessageID := sendComposeMessage(t, client, base, a.AccessToken, chatID, "tchat-unread-before-restart")
	require.Eventually(t, func() bool {
		item := composeA1ChatItem(listComposeChats(t, client, base, b.AccessToken, "main"), chatID)
		return item != nil && item.UnreadCount >= 2
	}, 45*time.Second, 500*time.Millisecond, "recipient must have durable unread metadata")
	markReadComposeMessage(t, client, base, b.AccessToken, chatID, secondMessageID)
	require.Equal(t, secondMessageID, getComposeReadState(t, client, base, b.AccessToken, chatID))

	thirdMessageID := sendComposeMessage(t, client, base, b.AccessToken, chatID, "tchat-history-cursor")
	requireTChatCursorHistory(t, client, base, a.AccessToken, chatID, []string{
		requestMessageID,
		secondMessageID,
		thirdMessageID,
	})

	state := tchatClosureState{
		ProofID:        proofID,
		CreatedAt:      time.Now().UnixNano(),
		ChatID:         chatID,
		AccountA:       a.AccountID,
		AccountB:       b.AccountID,
		ProfileA:       a.ProfileID,
		ProfileB:       b.ProfileID,
		AccessTokenA:   a.AccessToken,
		AccessTokenB:   b.AccessToken,
		HistoryMessage: []string{requestMessageID, secondMessageID, thirdMessageID},
	}
	require.GreaterOrEqual(t, state.CreatedAt, runStarted)
	writeTChatClosureState(t, statePath, state)
}

func verifyTChatActiveSocketClosureWithoutEvent(
	t *testing.T,
	client *http.Client,
	base, statePath, proofID string,
	runStarted int64,
) {
	t.Helper()
	state := readTChatClosureState(t, statePath)
	require.Equal(t, proofID, state.ProofID)
	require.GreaterOrEqual(t, state.CreatedAt, runStarted)
	waitTChatGatewayGETRoute(t, client, base+"/api/v1/chats?inbox=main&page_size=1", state.AccessTokenB)
	waitTChatGatewayGETRoute(t, client, base+"/api/v1/messages?chat_id="+state.ChatID+"&page_size=1", state.AccessTokenA)

	// The runner has stopped NATS, so BlockAccount can commit while its ignored
	// PublishUserBlocked error prevents this Realtime instance receiving an event.
	aDesktop := dialComposeRealtimeWS(t, base, state.AccessTokenA)
	aMobile := dialComposeRealtimeWS(t, base, state.AccessTokenA)
	b := dialComposeRealtimeWS(t, base, state.AccessTokenB)
	for _, conn := range []*websocket.Conn{aDesktop, aMobile, b} {
		waitComposeWSHello(t, conn)
		syncFrame := waitComposeWSOp(t, conn, "subscription_sync", 15*time.Second, nil)
		var syncData struct {
			ChatIDs  []string `json:"chat_ids"`
			Degraded bool     `json:"degraded"`
		}
		require.NoError(t, json.Unmarshal(syncFrame.D, &syncData))
		require.False(t, syncData.Degraded)
		require.Contains(t, syncData.ChatIDs, state.ChatID, "socket must subscribe before block")
	}

	blockComposeAccount(t, client, base, state.AccessTokenA, state.AccountB)
	require.Equal(t, http.StatusForbidden,
		sendComposeMessageStatus(t, client, base, state.AccessTokenA, state.ChatID, "blocked-active-a", ""))
	require.Equal(t, http.StatusForbidden,
		sendComposeMessageStatus(t, client, base, state.AccessTokenB, state.ChatID, "blocked-active-b", ""))

	for _, tc := range []struct {
		op   string
		d    map[string]any
		code string
	}{
		{op: "typing_start", d: map[string]any{"chat_id": state.ChatID}, code: "invalid_typing"},
		{op: "mark_read", d: map[string]any{"chat_id": state.ChatID, "message_id": state.HistoryMessage[2]}, code: "invalid_mark_read"},
		{op: "delivery_ack", d: map[string]any{"chat_id": state.ChatID, "message_id": state.HistoryMessage[2], "sender_profile_id": state.ProfileB}, code: "invalid_delivery_ack"},
	} {
		composeWSSend(t, aDesktop, map[string]any{"op": tc.op, "d": tc.d})
		waitComposeWSOp(t, aDesktop, "error", 15*time.Second, func(data map[string]any) bool {
			return data["code"] == tc.code
		})
	}
	requireNoTChatWSFrame(t, aMobile, "same-profile read side effect")
	requireNoTChatWSFrame(t, b, "blocked-peer typing/delivery side effect")
}

func requireNoTChatWSFrame(t *testing.T, conn *websocket.Conn, label string) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(500*time.Millisecond)))
	if _, _, err := conn.ReadMessage(); err == nil {
		t.Fatalf("unexpected %s", label)
	}
}

func verifyTChatBlockedDMTransportClosure(
	t *testing.T,
	client *http.Client,
	base, statePath, proofID string,
	runStarted int64,
) {
	t.Helper()
	state := readTChatClosureState(t, statePath)
	require.Equal(t, proofID, state.ProofID, "state must belong to this runner")
	require.GreaterOrEqual(t, state.CreatedAt, runStarted, "state predates this runner")
	require.Less(t, time.Since(time.Unix(0, state.CreatedAt)), 15*time.Minute, "state is stale")

	// Docker health can turn green before the restarted gRPC services have
	// republished their routes to Gateway. Wait only for transport readiness;
	// the acceptance assertions below still execute as separate requests.
	waitTChatGatewayGETRoute(t, client, base+"/api/v1/chats?inbox=main&page_size=1", state.AccessTokenB)
	waitTChatGatewayGETRoute(t, client, base+"/api/v1/messages?chat_id="+state.ChatID+"&page_size=1", state.AccessTokenA)

	// REST recovery is asserted independently from the WebSocket resume below.
	mainPage := listTChatInboxPage(t, client, base, state.AccessTokenB, "main", "", 1)
	require.Contains(t, mainPage.ChatIDs, state.ChatID)
	requireTChatCursorHistory(t, client, base, state.AccessTokenA, state.ChatID, state.HistoryMessage)
	require.Equal(t, http.StatusForbidden,
		sendComposeMessageStatus(t, client, base, state.AccessTokenA, state.ChatID, "blocked-after-restart-a", ""))
	require.Equal(t, http.StatusForbidden,
		sendComposeMessageStatus(t, client, base, state.AccessTokenB, state.ChatID, "blocked-after-restart-b", ""))

	ws := dialComposeRealtimeWS(t, base, state.AccessTokenB)
	waitComposeWSHello(t, ws)
	syncFrame := waitComposeWSOp(t, ws, "subscription_sync", 15*time.Second, nil)
	var syncData struct {
		ChatIDs  []string `json:"chat_ids"`
		Degraded bool     `json:"degraded"`
	}
	require.NoError(t, json.Unmarshal(syncFrame.D, &syncData))
	require.False(t, syncData.Degraded, "Realtime bootstrap dependency must be healthy after restart")
	assert.NotContains(t, syncData.ChatIDs, state.ChatID,
		"blocked DM must not be restored by fresh WS bootstrap")

	// `resume` is connection-local bookkeeping. It is intentionally separate
	// from the REST snapshot/history assertions above.
	composeWSSendResume(t, ws, syncFrame.S)
	composeWSSend(t, ws, map[string]any{
		"op": "subscribe",
		"d":  map[string]string{"chat_id": state.ChatID},
	})
	result := waitTChatSubscribeResult(t, ws, state.ChatID)
	assert.Equal(t, "error", result.Op,
		"lazy WS subscribe must not bypass the same block deny enforced by REST")
	if result.Op == "error" {
		var data map[string]any
		require.NoError(t, json.Unmarshal(result.D, &data))
		assert.Equal(t, "permission_denied", data["code"])
	}
}

func waitTChatGatewayGETRoute(t *testing.T, client *http.Client, endpoint, accessToken string) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	lastStatus := 0
	lastBody := ""
	for time.Now().Before(deadline) {
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		if err == nil {
			body, readErr := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.NoError(t, readErr)
			lastStatus = resp.StatusCode
			lastBody = string(body)
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	require.Equal(t, http.StatusOK, lastStatus,
		"Gateway route did not recover before acceptance checks; body=%s", lastBody)
}

type tchatInboxPage struct {
	ChatIDs    []string
	NextCursor string
}

func listTChatInboxPage(
	t *testing.T,
	client *http.Client,
	base, accessToken, inbox, cursor string,
	pageSize int,
) tchatInboxPage {
	t.Helper()
	endpoint := base + "/api/v1/chats?inbox=" + inbox + "&page_size=" + strconv.Itoa(pageSize)
	if cursor != "" {
		endpoint += "&cursor=" + cursor
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET inbox body=%s", string(body))
	var parsed struct {
		ChatList struct {
			Items []struct {
				Chat struct {
					ID string `json:"id"`
				} `json:"chat"`
			} `json:"items"`
			NextCursor string `json:"next_cursor"`
		} `json:"chat_list"`
	}
	require.NoError(t, json.Unmarshal(body, &parsed))
	result := tchatInboxPage{NextCursor: parsed.ChatList.NextCursor}
	for _, item := range parsed.ChatList.Items {
		result.ChatIDs = append(result.ChatIDs, item.Chat.ID)
	}
	return result
}

func requireTChatCursorHistory(
	t *testing.T,
	client *http.Client,
	base, accessToken, chatID string,
	wantIDs []string,
) {
	t.Helper()
	cursor := ""
	seen := make(map[string]bool, len(wantIDs))
	usedCursor := false
	for page := 0; page < len(wantIDs)+2; page++ {
		endpoint := base + "/api/v1/messages?chat_id=" + chatID + "&page_size=1"
		if cursor != "" {
			endpoint += "&cursor=" + cursor
			usedCursor = true
		}
		req, err := http.NewRequest(http.MethodGet, endpoint, nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		resp, err := client.Do(req)
		require.NoError(t, err)
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		require.NoError(t, readErr)
		require.Equal(t, http.StatusOK, resp.StatusCode, "GET history body=%s", string(body))
		var parsed struct {
			MessageList struct {
				Messages []struct {
					ID string `json:"id"`
				} `json:"messages"`
				NextCursor string `json:"next_cursor"`
			} `json:"message_list"`
		}
		require.NoError(t, json.Unmarshal(body, &parsed))
		for _, message := range parsed.MessageList.Messages {
			seen[message.ID] = true
		}
		cursor = parsed.MessageList.NextCursor
		if cursor == "" {
			break
		}
	}
	require.True(t, usedCursor, "history proof must consume a per-chat cursor")
	for _, id := range wantIDs {
		require.True(t, seen[id], "history must contain message %s", id)
	}
}

func waitTChatSubscribeResult(t *testing.T, conn interface {
	SetReadDeadline(time.Time) error
	ReadJSON(any) error
}, chatID string) composeWSFrame {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(15*time.Second)))
	for {
		var frame composeWSFrame
		require.NoError(t, conn.ReadJSON(&frame), "read WS subscribe result")
		if frame.Op != "subscribe_ack" && frame.Op != "error" {
			continue
		}
		var data map[string]any
		require.NoError(t, json.Unmarshal(frame.D, &data))
		if data["chat_id"] == chatID {
			return frame
		}
	}
}

func writeTChatClosureState(t *testing.T, statePath string, state tchatClosureState) {
	t.Helper()
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	file, err := os.OpenFile(statePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	require.NoError(t, err, "state path must be fresh")
	_, err = file.Write(encoded)
	require.NoError(t, err)
	require.NoError(t, file.Sync())
	require.NoError(t, file.Close())
}

func readTChatClosureState(t *testing.T, statePath string) tchatClosureState {
	t.Helper()
	encoded, err := os.ReadFile(statePath)
	require.NoError(t, err)
	var state tchatClosureState
	require.NoError(t, json.Unmarshal(encoded, &state))
	require.NotEmpty(t, state.ChatID)
	require.NotEmpty(t, state.AccessTokenA)
	require.NotEmpty(t, state.AccessTokenB)
	require.Len(t, state.HistoryMessage, 3)
	return state
}
