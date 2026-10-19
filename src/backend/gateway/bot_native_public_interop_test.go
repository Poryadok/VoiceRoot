package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	botv1 "voice.app/voice/bot/v1"
	chatv1 "voice.app/voice/chat/v1"
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/principal"
)

// The companion uses an actual Bot protected listener and independently built
// Role/Auth/Chat/Space fixtures. No test client replaces these service calls.
func TestBotNativeInstallActualHTTPToAllProtectedSources(t *testing.T) {
	dir := os.Getenv("BOT_GATEWAY_NATIVE_FIXTURE_DIR")
	if dir == "" {
		t.Skip("explicit private native public fixture only")
	}
	var ready struct {
		Address  string `json:"address"`
		Bot      string `json:"bot_id"`
		OtherBot string `json:"other_bot_id"`
		Account  string `json:"account_id"`
		Profile  string `json:"profile_id"`
		Space    string `json:"space_id"`
		Chat     string `json:"chat_id"`
		ChatDir  string `json:"chat_fixture_dir"`
		Key      string `json:"key_file"`
		Cert     string `json:"cert_file"`
	}
	require.Eventually(t, func() bool {
		raw, err := os.ReadFile(filepath.Join(dir, "ready.json"))
		return err == nil && json.Unmarshal(raw, &ready) == nil
	}, 35*time.Second, 40*time.Millisecond)
	defer func() { require.NoError(t, os.WriteFile(filepath.Join(dir, "done"), []byte("finished"), 0600)) }()
	key, err := principal.LoadRSAPrivateKeyFile(ready.Key)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{configRSAJWK("fixture", &key.PublicKey)}})
	}))
	t.Cleanup(jwks.Close)
	for suffix, value := range map[string]string{"GRPC_ADDR": ready.Address, "TLS_CA_FILE": ready.Cert, "TLS_SERVER_NAME": "localhost", "CLIENT_CERT_FILE": ready.Cert, "CLIENT_KEY_FILE": ready.Key} {
		t.Setenv("GATEWAY_BOT_ADMIN_AUDIT_"+suffix, value)
	}
	clients := &grpcClients{}
	clients.configureAuditInitiatingTransports()
	t.Cleanup(clients.close)
	require.Nil(t, clients.bot)
	require.NotNil(t, clients.botAuditAdmin)
	redis := miniredis.RunT(t)
	redis.Set("auth:session:min_epoch:"+ready.Account, "9")
	validator := voicejwt.NewJWKSValidator(jwks.URL, "voice-auth", "voice-client", voicejwt.WithHTTPClient(jwks.Client()))
	cfg := gatewayConfig{tokenValidator: validator, tokenBlacklist: newRedisTokenBlacklist(redis.Addr(), "", ""), blacklistRequired: true, sessionEpochStrict: true, sessionEpochFloor: newRedisSessionEpochFloor(redis.Addr(), ""), transcoder: newTranscoder(clients)}
	cfg.transcoder.lifecycleIssuer = issuer
	server := httptest.NewServer(newGateway(cfg))
	t.Cleanup(server.Close)
	payload := map[string]any{"sub": ready.Account, "user_id": ready.Account, "profile_id": ready.Profile, "session_epoch": int64(9), "account_type": "regular", "iss": "voice-auth", "aud": "voice-client", "jti": uuid.NewString(), "exp": time.Now().Add(time.Minute).Unix()}
	token := signConfigJWT(t, "fixture", key, payload)
	request := func(token, chat string, duplicate bool) (int, []byte) {
		body, err := json.Marshal(map[string]any{"allowed_chats": []map[string]string{{"id": chat, "type": "CHAT_TYPE_DM"}}})
		require.NoError(t, err)
		r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, server.URL+"/api/v1/bots/"+ready.Bot+"/spaces/"+ready.Space+"/install", bytes.NewReader(body))
		require.NoError(t, err)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		if duplicate {
			r.Header.Add("Authorization", "Bearer "+token)
		}
		r.Header.Set("X-Voice-User-Id", uuid.NewString())
		r.Header.Set("X-Voice-Profile-Id", uuid.NewString())
		r.Header.Set("X-Voice-Session-Epoch", "999")
		response, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		return response.StatusCode, raw
	}
	code, _ := request(token, ready.Chat, true)
	require.Equal(t, http.StatusUnauthorized, code)
	stale := cloneConfigJWTPayload(payload)
	stale["session_epoch"] = int64(8)
	code, _ = request(signConfigJWT(t, "fixture", key, stale), ready.Chat, false)
	require.Equal(t, http.StatusUnauthorized, code)
	code, body := request(token, ready.Chat, false)
	require.Equal(t, http.StatusOK, code, "actual HTTP Gateway -> Bot -> fresh Auth floor -> Role -> Chat -> native Space")
	var initial map[string]any
	require.NoError(t, json.Unmarshal(body, &initial))
	require.NotEmpty(t, initial)
	// The existing enabled route must reach the protected Bot caller and the
	// actual Role decision. A caller-supplied body cannot select another chat.
	for _, enabled := range []bool{false, true} {
		encoded, err := json.Marshal(map[string]any{"chat": map[string]any{"id": uuid.NewString()}, "space_id": ready.Space, "enabled": enabled})
		require.NoError(t, err)
		r, err := http.NewRequestWithContext(t.Context(), http.MethodPatch, server.URL+"/api/v1/bots/"+ready.Bot+"/chats/"+ready.Chat+"/enabled", bytes.NewReader(encoded))
		require.NoError(t, err)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Authorization", "Bearer "+token)
		response, err := http.DefaultClient.Do(r)
		require.NoError(t, err)
		raw, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, http.StatusNoContent, response.StatusCode, string(raw))
	}
	// This is the declared RPC adapter, not an invented HTTP route. Controlled
	// Gateway verified claims use the same actual actor/account just accepted by
	// HTTP above; the receiver still performs real mTLS/JWKS/fresh Java floor and
	// owner checks. The actor's manage-bots authority does not confer ownership.
	setter := &botv1.SetChatWhitelistRequest{BotId: ready.Bot, AllowedChats: []*chatv1.ChatRef{{Id: ready.Chat}}}
	_, err = clients.botAuditAdmin.SetChatWhitelist(context.Background(), setter)
	require.Equal(t, codes.Unauthenticated, status.Code(err), "actual protected RPC rejects absent delegated credential")
	setActor := httptest.NewRequest(http.MethodPost, "/", nil)
	setActor = setActor.WithContext(context.WithValue(setActor.Context(), verifiedUserClaimsKey{}, tokenClaims{UserID: ready.Account, ProfileID: ready.Profile, SessionEpoch: 9, AccountType: "regular", ExpiresAt: time.Now().Add(time.Minute)}))
	setCtx, err := cfg.transcoder.botLifecycleContext(setActor, setter, botv1.BotService_SetChatWhitelist_FullMethodName)
	require.NoError(t, err)
	_, err = clients.botAuditAdmin.SetChatWhitelist(setCtx, setter)
	require.NoError(t, err, "owner-only setter reaches actual protected receiver and durable Chat source")
	notOwned := &botv1.SetChatWhitelistRequest{BotId: ready.OtherBot, AllowedChats: setter.AllowedChats}
	setCtx, err = cfg.transcoder.botLifecycleContext(setActor, notOwned, botv1.BotService_SetChatWhitelist_FullMethodName)
	require.NoError(t, err)
	_, err = clients.botAuditAdmin.SetChatWhitelist(setCtx, notOwned)
	require.Equal(t, codes.PermissionDenied, status.Code(err), "a current Space manager cannot set another owner's whitelist")
	require.NoError(t, os.WriteFile(filepath.Join(ready.ChatDir, "retire-chat"), []byte("retire actual Chat after accepted install"), 0600))
	require.Eventually(t, func() bool { _, err := os.Stat(filepath.Join(ready.ChatDir, "retired-chat")); return err == nil }, 8*time.Second, 30*time.Millisecond)
	code, body = request(token, ready.Chat, false)
	require.Equal(t, http.StatusOK, code, "exact accepted install retry does not re-read deleted Chat")
	var again map[string]any
	require.NoError(t, json.Unmarshal(body, &again))
	require.Equal(t, initial, again)
	code, _ = request(token, uuid.NewString(), false)
	require.Equal(t, http.StatusNotFound, code, "a new unknown target is never admitted through old source evidence")
}
