package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestComposeDeveloperPortal_live exercises the real Voice authorization-code
// protocol and uses only the exchanged session to register and read back a bot.
// This HTTP journey does not claim browser sessionStorage/rendering coverage.
func TestComposeDeveloperPortal_live(t *testing.T) {
	if !liveComposeEnabled() {
		t.Skip("set VOICE_RUN_LIVE_COMPOSE=true to run against local compose")
	}

	client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	portalBase := liveDeveloperPortalBaseURL()
	gatewayBase := liveGatewayBaseURL()

	rootResp, err := client.Get(portalBase + "/")
	require.NoError(t, err)
	rootResp.Body.Close()
	require.Equal(t, http.StatusOK, rootResp.StatusCode, "developer portal must be available")

	callbackResp, err := client.Get(portalBase + "/callback")
	require.NoError(t, err)
	callbackResp.Body.Close()
	require.Equal(t, http.StatusOK, callbackResp.StatusCode, "portal /callback SPA route must be served")

	clientID := strings.TrimSpace(os.Getenv("DEVELOPER_PORTAL_OAUTH_CLIENT_ID"))
	if clientID == "" {
		clientID = "voice-developer-portal"
	}
	redirectURI := portalBase + "/callback"
	verifierBytes := make([]byte, 32)
	_, err = rand.Read(verifierBytes)
	require.NoError(t, err)
	verifier := base64.RawURLEncoding.EncodeToString(verifierBytes)
	challenge := sha256.Sum256([]byte(verifier))
	state := fmt.Sprintf("portal-%d", time.Now().UnixNano())
	form := url.Values{
		"response_type":         {"code"},
		"client_id":             {clientID},
		"redirect_uri":          {redirectURI},
		"state":                 {state},
		"code_challenge":        {base64.RawURLEncoding.EncodeToString(challenge[:])},
		"code_challenge_method": {"S256"},
	}
	authURL := gatewayBase + "/api/v1/auth/oauth2/authorize?" + form.Encode()
	oauthResp, err := client.Get(authURL)
	require.NoError(t, err)
	loginHTML, err := io.ReadAll(oauthResp.Body)
	oauthResp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, oauthResp.StatusCode)
	require.Contains(t, string(loginHTML), `action="/api/v1/auth/oauth2/authorize"`)

	// A redirect outside the configured allowlist must never receive a code.
	invalidForm, err := url.ParseQuery(form.Encode())
	require.NoError(t, err)
	invalidForm.Set("redirect_uri", "https://invalid.voice.test/callback")
	status, _ := portalForm(t, client, gatewayBase+"/api/v1/auth/oauth2/authorize", invalidForm)
	require.Equal(t, http.StatusBadRequest, status)

	email := formatComposeEmail("portal-oauth", time.Now().UnixNano())
	password := "VoicePortalTest1!"
	owner := registerComposeUser(t, client, gatewayBase, email, password)
	form.Set("email", email)
	form.Set("password", password+"wrong")
	status, _ = portalForm(t, client, gatewayBase+"/api/v1/auth/oauth2/authorize", form)
	require.Equal(t, http.StatusUnauthorized, status, "wrong password must not authorize a callback")
	form.Set("password", password)
	resp, err := client.PostForm(gatewayBase+"/api/v1/auth/oauth2/authorize", form)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	callback, err := resp.Location()
	require.NoError(t, err)
	require.Equal(t, redirectURI, callback.Scheme+"://"+callback.Host+callback.Path)
	require.Equal(t, state, callback.Query().Get("state"))
	code := callback.Query().Get("code")
	require.NotEmpty(t, code)

	exchange := url.Values{
		"grant_type": {"authorization_code"}, "client_id": {clientID},
		"redirect_uri": {redirectURI}, "code": {code},
		"code_verifier": {verifier + "wrong"},
		"client_secret": {os.Getenv("DEVELOPER_PORTAL_OAUTH_CLIENT_SECRET")},
	}
	status, raw := portalForm(t, client, gatewayBase+"/api/v1/auth/oauth2/token", exchange)
	require.Equal(t, http.StatusUnauthorized, status, "mismatched PKCE must not issue a session")
	var denied map[string]any
	require.NoError(t, json.Unmarshal(raw, &denied))
	require.Equal(t, "invalid_grant", denied["error"])
	exchange.Set("code_verifier", verifier)
	status, raw = portalForm(t, client, gatewayBase+"/api/v1/auth/oauth2/token", exchange)
	require.Equal(t, http.StatusOK, status)
	var session struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
	}
	require.NoError(t, json.Unmarshal(raw, &session))
	require.NotEmpty(t, session.AccessToken)
	require.Equal(t, "Bearer", session.TokenType)
	require.Positive(t, session.ExpiresIn)
	require.Empty(t, session.RefreshToken)
	status, _ = portalForm(t, client, gatewayBase+"/api/v1/auth/oauth2/token", exchange)
	require.Equal(t, http.StatusUnauthorized, status, "authorization codes are single-use")

	name := fmt.Sprintf("Portal-%d", time.Now().UnixNano())
	payload, err := json.Marshal(map[string]string{"name": name, "description": "Portal OAuth E2E", "scopes_json": `["TEXT_CHAT_SEND_MESSAGES"]`})
	require.NoError(t, err)
	status, _ = portalBotRequest(t, client, http.MethodPost, gatewayBase+"/api/v1/bots", "", payload)
	require.Equal(t, http.StatusUnauthorized, status, "registration requires authentication")
	status, raw = portalBotRequest(t, client, http.MethodPost, gatewayBase+"/api/v1/bots", session.AccessToken, payload)
	require.Equal(t, http.StatusOK, status)
	var registered struct {
		Bot struct {
			ID string `json:"id"`
		} `json:"bot"`
		TokenResponse struct {
			Token string `json:"token"`
		} `json:"token_response"`
		WebhookSecretResponse struct {
			WebhookSecret string `json:"webhook_secret"`
		} `json:"webhook_secret_response"`
	}
	require.NoError(t, json.Unmarshal(raw, &registered))
	require.NotEmpty(t, registered.Bot.ID)
	botURL := gatewayBase + "/api/v1/bots/" + registered.Bot.ID
	t.Cleanup(func() {
		status, _ := portalBotRequest(t, client, http.MethodDelete, botURL, session.AccessToken, nil)
		require.Equal(t, http.StatusNoContent, status, "remove disposable portal bot")
	})
	require.NotEmpty(t, registered.TokenResponse.Token, "registration must return the one-shot bot token")
	require.NotEmpty(t, registered.WebhookSecretResponse.WebhookSecret, "registration must return the webhook signing secret")
	status, raw = portalBotRequest(t, client, http.MethodGet, gatewayBase+"/api/v1/bots", session.AccessToken, nil)
	require.Equal(t, http.StatusOK, status)
	var listed struct {
		BotList struct {
			Bots []struct {
				ID string `json:"id"`
			} `json:"bots"`
		} `json:"bot_list"`
	}
	require.NoError(t, json.Unmarshal(raw, &listed))
	ids := make([]string, 0, len(listed.BotList.Bots))
	for _, bot := range listed.BotList.Bots {
		ids = append(ids, bot.ID)
	}
	require.Contains(t, ids, registered.Bot.ID, "portal owner list must include the saved bot")
	status, raw = portalBotRequest(t, client, http.MethodGet, botURL, session.AccessToken, nil)
	require.Equal(t, http.StatusOK, status)
	var saved struct {
		Bot struct {
			ID          string `json:"id"`
			Owner       string `json:"owner_account_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"bot"`
	}
	require.NoError(t, json.Unmarshal(raw, &saved))
	require.Equal(t, registered.Bot.ID, saved.Bot.ID)
	require.Equal(t, owner.AccountID, saved.Bot.Owner)
	require.Equal(t, name, saved.Bot.Name)
	require.Equal(t, "Portal OAuth E2E", saved.Bot.Description)
	t.Log("portal OAuth callback, PKCE session, single-use code, authenticated bot registration and owner readback passed")
}

func portalForm(t *testing.T, client *http.Client, endpoint string, form url.Values) (int, []byte) {
	t.Helper()
	resp, err := client.PostForm(endpoint, form)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	require.NoError(t, err)
	return resp.StatusCode, raw
}

func portalBotRequest(t *testing.T, client *http.Client, method, endpoint, token string, payload []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, endpoint, bytes.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	require.NoError(t, err)
	return resp.StatusCode, raw
}
