package principalruntime

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	botv1 "voice.app/voice/bot/v1"
	"voice/backend/pkg/principal"
)

func TestRuntimeConsumesCanonicalSpaceJWKSOverTLS(t *testing.T) {
	spaceKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	botKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	var keys []json.RawMessage
	for kid, key := range map[string]*rsa.PrivateKey{"current": spaceKey, "next": botKey} {
		response := httptest.NewRecorder()
		principal.JWKSHandler(kid, &key.PublicKey).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/.well-known/jwks.json", nil))
		var document struct {
			Keys []json.RawMessage `json:"keys"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &document); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, document.Keys...)
	}
	document, err := json.Marshal(struct {
		Keys []json.RawMessage `json:"keys"`
	}{keys})
	if err != nil {
		t.Fatal(err)
	}
	var canonicalRequests atomic.Int32
	// Space's dedicated HTTPS provider publishes its current/next public keys.
	// It does not require client authentication for JWKS (Space main.go).
	provider := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		canonicalRequests.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(document)
	}))
	t.Cleanup(provider.Close)
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	certificate := provider.TLS.Certificates[0]
	certFile := write("tls.pem", pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]}))
	tlsKey, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	tlsKeyFile := write("tls-key.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: tlsKey}))
	botPrivate, err := x509.MarshalPKCS8PrivateKey(botKey)
	if err != nil {
		t.Fatal(err)
	}
	botKeyFile := write("bot-key.pem", pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: botPrivate}))
	replay := miniredis.RunT(t)
	values := completeConfigEnv()
	for _, name := range []string{"GAME_INTEGRATION_PRINCIPAL_TLS_CERT_FILE", "GAME_INTEGRATION_PRINCIPAL_TLS_CA_FILE", "BOT_PRINCIPAL_TLS_CERT_FILE", "BOT_PRINCIPAL_TLS_CLIENT_CA_FILE", "BOT_GAME_EVENT_TLS_CERT_FILE", "BOT_GAME_EVENT_CLIENT_CA_FILE", "BOT_SPACE_LIFECYCLE_TLS_CERT_FILE", "BOT_SPACE_LIFECYCLE_CLIENT_CA_FILE", "SPACE_PRINCIPAL_JWKS_CA_FILE"} {
		values[name] = certFile
	}
	for _, name := range []string{"GAME_INTEGRATION_PRINCIPAL_TLS_KEY_FILE", "BOT_PRINCIPAL_TLS_KEY_FILE", "BOT_GAME_EVENT_TLS_KEY_FILE", "BOT_SPACE_LIFECYCLE_TLS_KEY_FILE"} {
		values[name] = tlsKeyFile
	}
	values["BOT_PRINCIPAL_PRIVATE_KEY_FILE"] = botKeyFile
	values["BOT_PRINCIPAL_KEY_ID"] = "current"
	values["BOT_PRINCIPAL_REPLAY_REDIS_URL"] = "redis://" + replay.Addr()
	values["BOT_SPACE_LIFECYCLE_GRPC_LISTEN"] = "127.0.0.1:0"
	values["SPACE_PRINCIPAL_JWKS_URL"] = provider.URL + "/.well-known/jwks.json"
	cfg, enabled, err := ConfigFromEnv(func(name string) string { return values[name] })
	if err != nil || !enabled {
		t.Fatalf("canonical Space provider configuration rejected: enabled=%v err=%v", enabled, err)
	}
	runtime, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = runtime.Close() })
	if canonicalRequests.Load() == 0 {
		t.Fatal("runtime did not fetch the canonical provider endpoint over trusted TLS")
	}
	issuer, err := principal.NewIssuer(principal.IssuerConfig{Issuer: "space", KeyID: "current", PrivateKey: spaceKey})
	if err != nil {
		t.Fatal(err)
	}
	req := &botv1.CheckBotInstallationScopeRequest{BotId: uuid.NewString(), SpaceId: uuid.NewString(), ActorProfileId: uuid.NewString(), InstallationId: uuid.NewString(), BotScope: "TEXT_CHAT_CREATE_IN_SPACE"}
	hash, err := principal.RequestHash(req)
	if err != nil {
		t.Fatal(err)
	}
	requestID := uuid.NewString()
	method := botv1.BotService_CheckBotInstallationScope_FullMethodName
	token, err := issuer.IssueService(principal.ServiceInput{Audience: "bot", RPC: method, RequestID: requestID, RequestHash: hash})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := runtime.VerifySpace(context.Background(), token, method, requestID, hash)
	if err != nil || verified.Issuer != "space" {
		t.Fatalf("actual fetched Space key rejected: %v", err)
	}
	if _, err := runtime.VerifySpace(context.Background(), token, method, requestID, hash); err == nil {
		t.Fatal("TLS JWKS compatibility bypassed single-use replay protection")
	}
}

func TestSpaceJWKSConfiguration_OnlyExactCanonicalOrLegacyHTTPS(t *testing.T) {
	for _, tc := range []struct {
		url     string
		allowed bool
	}{
		{"https://space:8443/.well-known/jwks.json", true},
		{"https://space:8443/space/jwks.json", true},
		{"http://space:8443/.well-known/jwks.json", false},
		{"https://user:password@space:8443/.well-known/jwks.json", false},
		{"https://space:8443/.well-known/jwks.json?source=other", false},
		{"https://space:8443/.well-known/jwks.json#fragment", false},
		{"https://space:8443/.well-known/jwks%2Ejson", false},
		{"https://space:8443/other/jwks.json", false},
	} {
		t.Run(tc.url, func(t *testing.T) {
			values := completeConfigEnv()
			values["BOT_SPACE_LIFECYCLE_GRPC_LISTEN"] = ":9444"
			values["BOT_SPACE_LIFECYCLE_TLS_CERT_FILE"] = "server.crt"
			values["BOT_SPACE_LIFECYCLE_TLS_KEY_FILE"] = "server.key"
			values["BOT_SPACE_LIFECYCLE_CLIENT_CA_FILE"] = "client-ca.crt"
			values["SPACE_PRINCIPAL_JWKS_CA_FILE"] = "space-ca.crt"
			values["SPACE_PRINCIPAL_JWKS_URL"] = tc.url
			_, enabled, err := ConfigFromEnv(func(name string) string { return values[name] })
			if !enabled || (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v err=%v", tc.allowed, err)
			}
		})
	}
}
