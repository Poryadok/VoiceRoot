package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	eventsv1 "voice.app/voice/events/v1"
)

func TestDecodeGuestConversionEventForAccount(t *testing.T) {
	const accountID = "620a7260-f990-4d68-9cb6-35d36067fb38"
	for _, tc := range []struct {
		name    string
		payload []byte
		want    bool
	}{
		{name: "malformed protobuf", payload: []byte{0xff}},
		{name: "legacy JSON", payload: []byte(`{"account_id":"` + accountID + `"}`)},
		{name: "unrelated account", payload: guestConversionPayload(t, "aa98c1e0-cdcd-45b1-8ab1-96374d49f4c8", true)},
		{name: "missing event identity", payload: guestConversionPayload(t, accountID, false)},
		{name: "current account", payload: guestConversionPayload(t, accountID, true), want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			event := decodeGuestConversionEventForAccount(tc.payload, accountID)
			require.Equal(t, tc.want, event != nil)
		})
	}
}

func guestConversionPayload(t *testing.T, accountID string, withEventID bool) []byte {
	t.Helper()
	event := &eventsv1.UserStreamEvent{
		OccurredAt: timestamppb.Now(),
		Payload: &eventsv1.UserStreamEvent_UserGuestConverted{
			UserGuestConverted: &eventsv1.UserGuestConverted{AccountId: accountID},
		},
	}
	if withEventID {
		event.EventId = "a9d94c61-14b4-4b8b-aa18-6010b6931679"
	}
	payload, err := proto.Marshal(event)
	require.NoError(t, err)
	return payload
}

func decodeGuestConversionEventForAccount(data []byte, accountID string) *eventsv1.UserStreamEvent {
	var event eventsv1.UserStreamEvent
	if err := proto.Unmarshal(data, &event); err != nil ||
		event.GetUserGuestConverted() == nil ||
		event.GetUserGuestConverted().GetAccountId() != accountID ||
		event.GetEventId() == "" ||
		event.GetOccurredAt() == nil || !event.GetOccurredAt().IsValid() {
		return nil
	}
	return &event
}

func liveNATSURL() string {
	if u := strings.TrimSpace(os.Getenv("VOICE_NATS_URL")); u != "" {
		return u
	}
	return "nats://127.0.0.1:4222"
}

// TestComposeConvertGuestNATS_live subscribes to core NATS subject user.guest_converted
// and asserts Auth durably publishes after email verification of a converted guest (AU-14).
//
// Opt-in: VOICE_RUN_LIVE_COMPOSE=true VOICE_API_BASE_URL=http://127.0.0.1:18080
func TestComposeConvertGuestNATS_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	nc, err := nats.Connect(liveNATSURL(), nats.Timeout(5*time.Second))
	require.NoError(t, err)
	defer nc.Close()

	sub, err := nc.SubscribeSync("user.guest_converted")
	require.NoError(t, err)
	defer sub.Unsubscribe()
	require.NoError(t, nc.FlushTimeout(5*time.Second))

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()

	const guestPassword = "VoiceQaTest1!"
	const newPassword = "VoiceQaNewPass1!"
	guestSess := registerComposeGuest(t, client, base, guestPassword)

	email := formatComposeEmail("guest-nats", time.Now().UnixNano())
	convertBody, err := json.Marshal(map[string]string{
		"email":    email,
		"password": newPassword,
	})
	require.NoError(t, err)
	convertReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/convert-guest", bytes.NewReader(convertBody))
	require.NoError(t, err)
	convertReq.Header.Set("Authorization", "Bearer "+guestSess.AccessToken)
	convertReq.Header.Set("Content-Type", "application/json")
	convertResp, err := client.Do(convertReq)
	require.NoError(t, err)
	defer convertResp.Body.Close()
	convertRaw, _ := io.ReadAll(convertResp.Body)
	require.Equal(t, http.StatusOK, convertResp.StatusCode, "body=%s", string(convertRaw))

	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(convertRaw, &envelope))
	pending := envelope.Session
	require.Equal(t, guestSess.AccountID, pending.AccountID)
	require.Equal(t, "guest", pending.AccountType, "conversion remains pending until email verification")
	verified := completeComposeEmailVerification(t, client, base, email, pending)
	require.Equal(t, guestSess.AccountID, verified.AccountID)

	// Recovery scans every 30 seconds; leave room for a delayed scan and broker delivery.
	deadline := time.Now().Add(45 * time.Second)
	matched := false
	for time.Until(deadline) > 0 {
		msg, err := sub.NextMsg(time.Until(deadline))
		if errors.Is(err, nats.ErrTimeout) {
			break
		}
		require.NoError(t, err, "read user.guest_converted from core NATS")
		event := decodeGuestConversionEventForAccount(msg.Data, guestSess.AccountID)
		if event == nil {
			continue
		}
		require.Equal(t, event.GetEventId(), msg.Header.Get("Nats-Msg-Id"),
			"guest conversion must use the durable JetStream operation identity")
		require.False(t, matched, "duplicate guest conversion event for account %s", guestSess.AccountID)
		matched = true
		deadline = time.Now().Add(time.Second)
	}
	require.True(t, matched, "expected durable user.guest_converted for account %s on core NATS", guestSess.AccountID)
}

// TestComposeAuthSessions_live: list sessions → revoke other → refresh fails (AU-12).
func TestComposeAuthSessions_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}
	clearLiveComposeAuthRateLimit(t)

	client := &http.Client{Timeout: 45 * time.Second}
	base := liveGatewayBaseURL()
	n := time.Now().UnixNano()
	email := formatComposeEmail("auth-sess", n)
	password := "VoiceQaTest1!"

	first := registerComposeUser(t, client, base, email, password)
	second := loginComposeUser(t, client, base, email, password)

	listReq, err := http.NewRequest(http.MethodGet, base+"/api/v1/auth/sessions", nil)
	require.NoError(t, err)
	listReq.Header.Set("Authorization", "Bearer "+second.AccessToken)
	listResp, err := client.Do(listReq)
	require.NoError(t, err)
	defer listResp.Body.Close()
	listRaw, _ := io.ReadAll(listResp.Body)
	require.Equal(t, http.StatusOK, listResp.StatusCode, "body=%s", string(listRaw))

	var list struct {
		Sessions []struct {
			ID      string `json:"id"`
			Current bool   `json:"current"`
		} `json:"sessions"`
	}
	require.NoError(t, json.Unmarshal(listRaw, &list))
	require.GreaterOrEqual(t, len(list.Sessions), 2)

	var otherID string
	for _, s := range list.Sessions {
		if !s.Current {
			otherID = s.ID
			break
		}
	}
	require.NotEmpty(t, otherID)

	revokeReq, err := http.NewRequest(http.MethodPost, base+"/api/v1/auth/sessions/"+otherID+"/revoke", nil)
	require.NoError(t, err)
	revokeReq.Header.Set("Authorization", "Bearer "+second.AccessToken)
	revokeResp, err := client.Do(revokeReq)
	require.NoError(t, err)
	defer revokeResp.Body.Close()
	require.Equal(t, http.StatusNoContent, revokeResp.StatusCode)

	refreshPayload, err := json.Marshal(map[string]string{
		"refresh_token":    first.RefreshToken,
		"device_info_json": `{"platform":"go-live-test"}`,
	})
	require.NoError(t, err)
	refreshResp, err := client.Post(base+"/api/v1/auth/refresh", "application/json", bytes.NewReader(refreshPayload))
	require.NoError(t, err)
	defer refreshResp.Body.Close()
	refreshRaw, _ := io.ReadAll(refreshResp.Body)
	require.Equal(t, http.StatusUnauthorized, refreshResp.StatusCode, "body=%s", string(refreshRaw))
}

func loginComposeUser(t *testing.T, client *http.Client, base, email, password string) authSessionResponse {
	t.Helper()
	payload, err := json.Marshal(map[string]any{
		"email":            email,
		"password":         password,
		"device_info_json": `{"platform":"go-live-test-b"}`,
	})
	require.NoError(t, err)
	resp, err := client.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "login body=%s", string(raw))
	var envelope authSessionEnvelope
	require.NoError(t, json.Unmarshal(raw, &envelope))
	require.NotEmpty(t, envelope.Session.AccessToken)
	require.NotEmpty(t, envelope.Session.RefreshToken)
	return envelope.Session
}
