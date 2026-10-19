package main

import (
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
	voicejwt "voice/backend/pkg/jwt"
	"voice/backend/pkg/principal"
)

// This companion requires the explicitly running real Bot/Java/PostgreSQL
// fixture. It never replaces the Bot client with a fake implementation.
func TestBotLifecycleActualHTTPToProtectedBot(t *testing.T) {
	dir := os.Getenv("BOT_GATEWAY_HTTP_FIXTURE_DIR")
	if dir == "" {
		t.Skip("explicit cross-process fixture only")
	}
	data, err := os.ReadFile(filepath.Join(dir, "ready.json"))
	require.NoError(t, err)
	var ready struct {
		Address   string `json:"address"`
		BotID     string `json:"bot_id"`
		AccountID string `json:"account_id"`
		ProfileID string `json:"profile_id"`
		KeyFile   string `json:"key_file"`
		CertFile  string `json:"cert_file"`
		Expect    int    `json:"expect"`
	}
	require.NoError(t, json.Unmarshal(data, &ready))
	defer func() { require.NoError(t, os.WriteFile(filepath.Join(dir, "done"), []byte("finished"), 0600)) }()
	key, err := principal.LoadRSAPrivateKeyFile(ready.KeyFile)
	require.NoError(t, err)
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "gateway", KeyID: "fixture", PrivateKey: key})
	require.NoError(t, err)
	jwks := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{configRSAJWK("fixture", &key.PublicKey)}})
	}))
	t.Cleanup(jwks.Close)
	for suffix, value := range map[string]string{"GRPC_ADDR": ready.Address, "TLS_CA_FILE": ready.CertFile, "TLS_SERVER_NAME": "localhost", "CLIENT_CERT_FILE": ready.CertFile, "CLIENT_KEY_FILE": ready.KeyFile} {
		t.Setenv("GATEWAY_BOT_ADMIN_AUDIT_"+suffix, value)
	}
	clients := &grpcClients{}
	clients.configureAuditInitiatingTransports()
	t.Cleanup(clients.close)
	require.Nil(t, clients.bot, "ordinary Bot client must not be required")
	require.NotNil(t, clients.botAuditAdmin)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	require.NoError(t, clients.waitAuditInitiatingReady(ctx))
	redis := miniredis.RunT(t)
	redis.Set("auth:session:min_epoch:"+ready.AccountID, "9")
	validator := voicejwt.NewJWKSValidator(jwks.URL, "voice-auth", "voice-client", voicejwt.WithHTTPClient(jwks.Client()))
	cfg := gatewayConfig{tokenValidator: validator, tokenBlacklist: newRedisTokenBlacklist(redis.Addr(), "", ""), blacklistRequired: true, sessionEpochStrict: true, sessionEpochFloor: newRedisSessionEpochFloor(redis.Addr(), ""), transcoder: newTranscoder(clients)}
	cfg.transcoder.lifecycleIssuer = issuer
	httpServer := httptest.NewServer(newGateway(cfg))
	t.Cleanup(httpServer.Close)
	payload := map[string]any{"sub": ready.AccountID, "user_id": ready.AccountID, "profile_id": ready.ProfileID, "session_epoch": int64(9), "account_type": "regular", "iss": "voice-auth", "aud": "voice-client", "jti": uuid.NewString(), "exp": time.Now().Add(time.Minute).Unix()}
	request := func(token string, duplicate bool) int {
		r, err := http.NewRequestWithContext(t.Context(), http.MethodDelete, httpServer.URL+"/api/v1/bots/"+ready.BotID, nil)
		require.NoError(t, err)
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
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		return response.StatusCode
	}
	token := signConfigJWT(t, "fixture", key, payload)
	require.Equal(t, http.StatusUnauthorized, request(token, true))
	stale := cloneConfigJWTPayload(payload)
	stale["session_epoch"] = int64(8)
	require.Equal(t, http.StatusUnauthorized, request(signConfigJWT(t, "fixture", key, stale), false))
	require.Equal(t, ready.Expect, request(token, false), "actual HTTP -> signed Gateway -> mTLS Bot -> fresh Java floor -> PostgreSQL decision")
}
