package gameconsent

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestConfigFromEnvFailsClosedOnPartialOrInvalidConfig(t *testing.T) {
	values := map[string]string{}
	getenv := func(name string) string { return values[name] }
	client, enabled, err := ConfigFromEnv(getenv)
	require.NoError(t, err)
	require.False(t, enabled)
	require.Nil(t, client)
	values["GAME_INTEGRATION_CONSENT_URL"] = "http://gameintegration:8080"
	_, _, err = ConfigFromEnv(getenv)
	require.Error(t, err, "partial authority configuration must not silently enable unguarded game pushes")
	values["GAME_INTEGRATION_NOTIFICATION_WORKLOAD_KEY_B64"] = base64.StdEncoding.EncodeToString([]byte("abcdef0123456789abcdef0123456789"))
	client, enabled, err = ConfigFromEnv(getenv)
	require.NoError(t, err)
	require.True(t, enabled)
	require.NotNil(t, client)
}

func TestClientSignsScopeAndVerifiesGISResponse(t *testing.T) {
	key := []byte("abcdef0123456789abcdef0123456789")
	now := time.Now().UTC().Truncate(time.Second)
	appID, envID, profileID := uuid.New(), uuid.New(), uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "notification", r.Header.Get("X-Voice-Workload"))
		require.Equal(t, "GET", r.Method)
		timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
		require.Equal(t, strconv.FormatInt(now.Unix(), 10), timestamp)
		require.Equal(t, requestSignature(key, r.Method, r.URL.EscapedPath(), timestamp, nonce), r.Header.Get("X-Voice-Signature"))
		require.Equal(t, fmt.Sprintf("/internal/v1/notification-consents/%s/%s/%s/game_activity", appID, envID, profileID), r.URL.Path)
		body := []byte(`{"allowed":true}`)
		w.Header().Set("X-Voice-Response-Timestamp", timestamp)
		w.Header().Set("X-Voice-Response-Nonce", nonce)
		w.Header().Set("X-Voice-Response-Signature", responseSignature(key, http.StatusOK, r.URL.EscapedPath(), timestamp, nonce, body))
		_, _ = w.Write(body)
	}))
	defer server.Close()
	client, err := New(server.URL, key)
	require.NoError(t, err)
	client.Now = func() time.Time { return now }
	client.NewNonce = func() string { return "22222222-2222-4222-8222-222222222222" }
	allowed, err := client.AllowsGamePush(context.Background(), profileID, appID, envID, "game_activity")
	require.NoError(t, err)
	require.True(t, allowed)
}

func TestClientFailsClosedOnUnsignedAuthorityResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(`{"allowed":true}`)) }))
	defer server.Close()
	client, err := New(server.URL, []byte("abcdef0123456789abcdef0123456789"))
	require.NoError(t, err)
	allowed, err := client.AllowsGamePush(context.Background(), uuid.New(), uuid.New(), uuid.New(), "game_activity")
	require.ErrorIs(t, err, ErrUnavailable)
	require.False(t, allowed)
}

func TestClientResolvesChatScopeForPush(t *testing.T) {
	key := []byte("abcdef0123456789abcdef0123456789")
	now := time.Now().UTC().Truncate(time.Second)
	profileID, chatID := uuid.New(), uuid.New()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timestamp, nonce := r.Header.Get("X-Voice-Timestamp"), r.Header.Get("X-Voice-Nonce")
		require.Equal(t, "/internal/v1/notification-consents/chat/"+profileID.String()+"/"+chatID.String()+"/game_activity/push", r.URL.Path)
		require.Equal(t, requestSignature(key, http.MethodGet, r.URL.EscapedPath(), timestamp, nonce), r.Header.Get("X-Voice-Signature"))
		body := []byte(`{"game_scoped":true,"allowed":false}`)
		w.Header().Set("X-Voice-Response-Timestamp", timestamp)
		w.Header().Set("X-Voice-Response-Nonce", nonce)
		w.Header().Set("X-Voice-Response-Signature", responseSignature(key, http.StatusOK, r.URL.EscapedPath(), timestamp, nonce, body))
		_, _ = w.Write(body)
	}))
	defer server.Close()
	client, err := New(server.URL, key)
	require.NoError(t, err)
	client.Now = func() time.Time { return now }
	client.NewNonce = func() string { return "22222222-2222-4222-8222-222222222222" }
	gameScoped, allowed, err := client.ResolveGamePushChat(context.Background(), profileID, chatID, "game_activity")
	require.NoError(t, err)
	require.True(t, gameScoped)
	require.False(t, allowed)
}
