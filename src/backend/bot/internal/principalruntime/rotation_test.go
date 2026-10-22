package principalruntime

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"voice/backend/pkg/principal"
)

func rotationRSAKey(t *testing.T, bits int) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func writeRotationKey(t *testing.T, dir, kid string, key *rsa.PrivateKey) {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, kid+".pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCanonicalRotationConfigRejectsPartialAndMixedModes(t *testing.T) {
	for _, name := range []string{"BOT_PRINCIPAL_SIGNING_KEYS_DIR", "BOT_PRINCIPAL_ACTIVE_KID"} {
		t.Run(name, func(t *testing.T) {
			values := completeConfigEnv()
			values[name] = "configured"
			if _, enabled, err := ConfigFromEnv(func(key string) string { return values[key] }); !enabled || err == nil {
				t.Fatal("partial canonical config ignored")
			}
		})
	}
	values := completeConfigEnv()
	values["BOT_PRINCIPAL_SIGNING_KEYS_DIR"] = "keys"
	values["BOT_PRINCIPAL_ACTIVE_KID"] = "current"
	if _, _, err := ConfigFromEnv(func(key string) string { return values[key] }); err == nil {
		t.Fatal("mixed signer modes accepted")
	}
	values = map[string]string{"BOT_PRINCIPAL_SIGNING_KEYS_DIR": "keys", "BOT_PRINCIPAL_ACTIVE_KID": "current"}
	if _, enabled, err := ConfigFromEnv(func(key string) string { return values[key] }); !enabled || err == nil {
		t.Fatal("canonical-only partial runtime treated as disabled")
	}
}

func TestCanonicalRotationRejectsInvalidDirectory(t *testing.T) {
	current, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	next, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing peer", "duplicate modulus", "missing active", "extra key", "invalid PEM", "PKCS1", "non RSA", "invalid kid", "weak RSA", "outside symlink"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeRotationKey(t, dir, "current", current)
			writeRotationKey(t, dir, "next", next)
			cfg := Config{BotSigningKeysDir: dir, BotActiveKID: "current"}
			switch name {
			case "missing peer":
				if err := os.Remove(filepath.Join(dir, "next.pem")); err != nil {
					t.Fatal(err)
				}
			case "duplicate modulus":
				writeRotationKey(t, dir, "next", current)
			case "missing active":
				cfg.BotActiveKID = "absent"
			case "extra key":
				writeRotationKey(t, dir, "extra", next)
			case "invalid PEM":
				if err := os.WriteFile(filepath.Join(dir, "next.pem"), []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			case "PKCS1":
				if err := os.WriteFile(filepath.Join(dir, "next.pem"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(next)}), 0600); err != nil {
					t.Fatal(err)
				}
			case "invalid kid":
				cfg.BotActiveKID = "../current"
			case "non RSA":
				key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				if err != nil {
					t.Fatal(err)
				}
				der, err := x509.MarshalPKCS8PrivateKey(key)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "next.pem"), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0600); err != nil {
					t.Fatal(err)
				}
			case "weak RSA":
				weak, err := rsa.GenerateKey(rand.Reader, 1024)
				if err != nil {
					t.Fatal(err)
				}
				writeRotationKey(t, dir, "next", weak)
			case "outside symlink":
				outside := t.TempDir()
				writeRotationKey(t, outside, "next", next)
				if err := os.Remove(filepath.Join(dir, "next.pem")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(outside, "next.pem"), filepath.Join(dir, "next.pem")); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, _, err := loadBotSigningKeys(cfg); err == nil {
				t.Fatal("invalid rotation directory accepted")
			}
		})
	}
}

func TestCanonicalRotationSelectsActiveAndPreservesLegacySigner(t *testing.T) {
	dir := t.TempDir()
	current := rotationRSAKey(t, 2048)
	next := rotationRSAKey(t, 2048)
	writeRotationKey(t, dir, "current", current)
	writeRotationKey(t, dir, "next", next)
	key, kid, keys, err := loadBotSigningKeys(Config{BotSigningKeysDir: dir, BotActiveKID: "next"})
	if err != nil || kid != "next" || key.N.Cmp(next.N) != 0 || len(keys) != 2 {
		t.Fatalf("active peer selection failed: kid=%s err=%v", kid, err)
	}
	key, kid, keys, err = loadBotSigningKeys(Config{BotPrivateKeyFile: filepath.Join(dir, "current.pem"), BotKeyID: "legacy-kid"})
	if err != nil || kid != "legacy-kid" || key.N.Cmp(current.N) != 0 || len(keys) != 1 || keys[0].KeyID != "legacy-kid" {
		t.Fatalf("legacy signer changed: kid=%s err=%v", kid, err)
	}
}

func TestCanonicalRotationActualOwnerEndpoint(t *testing.T) {
	current := rotationRSAKey(t, 2048)
	next := rotationRSAKey(t, 2048)
	dir := t.TempDir()
	writeRotationKey(t, dir, "current", current)
	writeRotationKey(t, dir, "next", next)
	// A real owner runtime with its Redis dependency and a locally trusted mTLS identity.
	certKey := rotationRSAKey(t, 2048)
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "bot"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &certKey.PublicKey, certKey)
	if err != nil {
		t.Fatal(err)
	}
	certPath := filepath.Join(t.TempDir(), "cert.pem")
	keyPath := filepath.Join(t.TempDir(), "key.pem")
	if err = os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0600); err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(certKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), 0600); err != nil {
		t.Fatal(err)
	}
	redis := miniredis.RunT(t)
	values := completeConfigEnv()
	delete(values, "BOT_PRINCIPAL_PRIVATE_KEY_FILE")
	delete(values, "BOT_PRINCIPAL_KEY_ID")
	values["BOT_PRINCIPAL_SIGNING_KEYS_DIR"] = dir
	values["BOT_PRINCIPAL_ACTIVE_KID"] = "current"
	values["BOT_PRINCIPAL_REPLAY_REDIS_URL"] = "redis://" + redis.Addr()
	for _, name := range []string{"GAME_INTEGRATION_PRINCIPAL_TLS_CERT_FILE", "GAME_INTEGRATION_PRINCIPAL_TLS_CA_FILE", "BOT_PRINCIPAL_TLS_CERT_FILE", "BOT_PRINCIPAL_TLS_CLIENT_CA_FILE"} {
		values[name] = certPath
	}
	values["GAME_INTEGRATION_PRINCIPAL_TLS_KEY_FILE"] = keyPath
	values["BOT_PRINCIPAL_TLS_KEY_FILE"] = keyPath
	cfg, enabled, err := ConfigFromEnv(func(key string) string { return values[key] })
	if err != nil || !enabled {
		t.Fatalf("canonical config enabled=%v err=%v", enabled, err)
	}
	runtime, err := New(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	owner, err := runtime.JWKSHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(owner.Handler)
	server.TLS = owner.TLSConfig.Clone()
	server.StartTLS()
	defer server.Close()
	clientCert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})) {
		t.Fatal("invalid test CA")
	}
	client := server.Client()
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots, Certificates: []tls.Certificate{clientCert}}
	response, err := client.Get(server.URL + "/.well-known/principal-jwks.json")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	keys, err := principal.ParseJWKS(body)
	if err != nil || len(keys) != 2 {
		t.Fatalf("owner JWKS status=%d keys=%d err=%v", response.StatusCode, len(keys), err)
	}
	if keys["current"] == nil || keys["next"] == nil || keys["current"].N.Cmp(current.N) != 0 || keys["next"].N.Cmp(next.N) != 0 {
		t.Fatal("owner published wrong current/next keys")
	}
	if keys["current"].N.Cmp(keys["next"].N) == 0 || strings.Contains(string(body), "PRIVATE") || strings.Contains(string(body), `"d"`) {
		t.Fatal("invalid public rotation set")
	}
	token, err := runtime.Issuer().IssueService(principal.ServiceInput{Audience: "chat", RPC: "/voice.chat.v1.ChatService/CreateChat", RequestID: "rotation-proof", RequestHash: "sha256:" + strings.Repeat("0", 64)})
	if err != nil {
		t.Fatal(err)
	}
	verified, err := principal.VerifyService(context.Background(), token, principal.VerifyConfig{ExpectedIssuer: "bot", ExpectedAudience: "chat", ExpectedRPC: "/voice.chat.v1.ChatService/CreateChat", ExpectedRequestID: "rotation-proof", ExpectedRequestHash: "sha256:" + strings.Repeat("0", 64), KeyResolver: func(_ context.Context, issuer, kid string) (*rsa.PublicKey, error) {
		if kid != "current" {
			t.Fatalf("signing switched to %s", kid)
		}
		return keys[kid], nil
	}, ReplayGuard: func(context.Context, string, string, time.Time) error { return nil }})
	if err != nil || verified.Issuer != "bot" {
		t.Fatalf("current signature rejected: %v", err)
	}
}
